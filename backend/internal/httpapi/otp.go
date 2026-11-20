package httpapi

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog/log"

	"github.com/masterplotter/loyalty/internal/store"
)

const otpTTL = 5 * time.Minute

func (a *API) otpHash(customerID int64, code string) []byte {
	m := hmac.New(sha256.New, []byte(a.cfg.JWTSecret))
	m.Write([]byte("otp|" + strconv.FormatInt(customerID, 10) + "|" + code))
	return m.Sum(nil)
}

func newOTP() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

// notifyAsync: kirim pesan tanpa menahan respons; gagal hanya di-log.
func (a *API) notifyAsync(phone, text string) {
	if a.sender == nil {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := a.sender.Send(ctx, phone, text); err != nil {
			log.Warn().Err(err).Msg("kirim notifikasi whatsapp gagal")
		}
	}()
}

// POST /me/verify/request — kirim OTP 6 digit ke WhatsApp pelanggan.
func (a *API) otpRequest(c *fiber.Ctx) error {
	if a.sender == nil {
		return fiber.NewError(fiber.StatusServiceUnavailable, "verifikasi otomatis belum tersedia, hubungi Master Plotter")
	}
	cu := customerFrom(c)
	if cu.PhoneVerified {
		return conflict("nomor sudah terverifikasi")
	}
	code, err := newOTP()
	if err != nil {
		return err
	}
	err = a.st.CreateOTP(c.Context(), cu.ID, a.otpHash(cu.ID, code), otpTTL)
	switch {
	case errors.Is(err, store.ErrOTPTooSoon):
		return fiber.NewError(fiber.StatusTooManyRequests, "tunggu 1 menit sebelum meminta kode baru")
	case errors.Is(err, store.ErrOTPTooMany):
		return fiber.NewError(fiber.StatusTooManyRequests, "terlalu banyak permintaan, coba lagi nanti")
	case err != nil:
		return err
	}
	ctx, cancel := context.WithTimeout(c.Context(), 12*time.Second)
	defer cancel()
	msg := fmt.Sprintf("Kode verifikasi Master Plotter: %s. Berlaku 5 menit. Jangan bagikan kode ini ke siapa pun.", code)
	if err := a.sender.Send(ctx, cu.Phone, msg); err != nil {
		log.Error().Err(err).Msg("kirim OTP gagal")
		return fiber.NewError(fiber.StatusBadGateway, "gagal mengirim kode, coba lagi")
	}
	return c.JSON(fiber.Map{"sent": true, "expires_in": int(otpTTL.Seconds())})
}

type otpConfirmReq struct {
	Code string `json:"code" validate:"required,len=6,numeric"`
}

// POST /me/verify/confirm
func (a *API) otpConfirm(c *fiber.Ctx) error {
	var req otpConfirmReq
	if err := a.parse(c, &req); err != nil {
		return err
	}
	cu := customerFrom(c)
	err := a.st.VerifyOTP(c.Context(), cu.ID, a.otpHash(cu.ID, req.Code))
	switch {
	case errors.Is(err, store.ErrOTPInvalid):
		return bad("kode salah atau sudah kedaluwarsa")
	case errors.Is(err, store.ErrOTPLocked):
		return fiber.NewError(fiber.StatusTooManyRequests, "terlalu banyak percobaan, minta kode baru")
	case err != nil:
		return err
	}
	return c.JSON(fiber.Map{"phone_verified": true})
}
