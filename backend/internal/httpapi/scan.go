package httpapi

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/masterplotter/loyalty/internal/phone"
	"github.com/masterplotter/loyalty/internal/store"
)

const (
	deviceCookie = "mp_device"
	deviceTTL    = 365 * 24 * time.Hour
)

func hashToken(t string) []byte { h := sha256.Sum256([]byte(t)); return h[:] }

func newDeviceToken() (token string, hash []byte, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return
	}
	token = base64.RawURLEncoding.EncodeToString(b)
	return token, hashToken(token), nil
}

// deviceCustomer mengenali pembeli dari cookie perangkat (nil jika tidak dikenali).
func (a *API) deviceCustomer(c *fiber.Ctx) *store.Customer {
	tok := c.Cookies(deviceCookie)
	if tok == "" {
		return nil
	}
	cust, err := a.st.CustomerByDevice(c.Context(), hashToken(tok))
	if err != nil {
		return nil
	}
	return &cust
}

// GET /scan/:code — status publik kode. Kode asal-asalan/belum aktif/void semuanya "invalid".
func (a *API) scanInfo(c *fiber.Ctx) error {
	cust := a.deviceCustomer(c)
	resp := fiber.Map{"state": "invalid", "known": cust != nil}
	if cust != nil {
		resp["name"] = cust.Name
	}
	code, err := uuid.Parse(c.Params("code"))
	if err != nil {
		return c.JSON(resp)
	}
	var cid int64
	if cust != nil {
		cid = cust.ID
	}
	info := a.st.ScanInfo(c.Context(), code, cid)
	resp["state"] = info.State
	switch info.State {
	case "ok":
		resp["product"], resp["points"] = info.ProductName, info.Points
	case "claimed":
		resp["mine"] = info.Mine
	}
	c.Set(fiber.HeaderCacheControl, "no-store")
	return c.JSON(resp)
}

type claimReq struct {
	Name    string `json:"name"`
	Phone   string `json:"phone"`
	Consent bool   `json:"consent"`
}

// POST /scan/:code/claim
func (a *API) claim(c *fiber.Ctx) error {
	code, err := uuid.Parse(c.Params("code"))
	if err != nil {
		return fiber.NewError(fiber.StatusNotFound, "kode tidak valid")
	}
	var req claimReq
	if len(c.Body()) > 0 {
		if err := c.BodyParser(&req); err != nil {
			return bad("body tidak valid")
		}
	}
	in := store.ClaimInput{Code: code, IP: c.IP(), UA: c.Get(fiber.HeaderUserAgent), Consent: req.Consent}
	if tok := c.Cookies(deviceCookie); tok != "" {
		in.DeviceHash = hashToken(tok)
	}
	if req.Name != "" || req.Phone != "" {
		name := strings.Join(strings.Fields(req.Name), " ")
		if n := utf8.RuneCountInString(name); n < 2 || n > 60 {
			return bad("nama harus 2-60 karakter")
		}
		p, err := phone.Normalize(req.Phone)
		if err != nil {
			return bad(err.Error())
		}
		in.Name, in.Phone = name, p
	}
	token, hash, err := newDeviceToken()
	if err != nil {
		return err
	}
	in.NewDeviceHash = hash

	res, err := a.st.Claim(c.Context(), in)
	switch {
	case errors.Is(err, store.ErrNotFound):
		return fiber.NewError(fiber.StatusNotFound, "kode tidak valid")
	case errors.Is(err, store.ErrClaimed):
		return conflict("kode ini sudah digunakan")
	case errors.Is(err, store.ErrNeedProfile):
		return bad("nama, nomor WhatsApp, dan persetujuan wajib diisi")
	case err != nil:
		return err
	}
	if res.NewDevice {
		c.Cookie(&fiber.Cookie{
			Name: deviceCookie, Value: token, Path: "/", HTTPOnly: true,
			Secure: a.cfg.CookieSecure, SameSite: "Lax",
			Expires: time.Now().Add(deviceTTL), MaxAge: int(deviceTTL.Seconds()),
		})
	}
	return c.JSON(fiber.Map{
		"earned": res.Earned, "balance": res.Balance, "name": res.Name,
		"product": res.Product, "new_customer": res.NewCustomer,
	})
}

func (a *API) requireCustomer(c *fiber.Ctx) error {
	cust := a.deviceCustomer(c)
	if cust == nil {
		return fiber.NewError(fiber.StatusUnauthorized, "scan QR untuk memulai")
	}
	c.Locals("customer", cust)
	return c.Next()
}

func customerFrom(c *fiber.Ctx) *store.Customer { return c.Locals("customer").(*store.Customer) }

func (a *API) customerMe(c *fiber.Ctx) error {
	cu := customerFrom(c)
	return c.JSON(fiber.Map{
		"name": cu.Name, "points": cu.Points, "phone": phone.Mask(cu.Phone), "phone_verified": cu.PhoneVerified,
		"otp_available": a.sender != nil,
	})
}

func (a *API) customerLedger(c *fiber.Ctx) error {
	es, err := a.st.Ledger(c.Context(), customerFrom(c).ID, 50)
	if err != nil {
		return err
	}
	return c.JSON(es)
}
