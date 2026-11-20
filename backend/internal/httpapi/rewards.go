package httpapi

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/gofiber/fiber/v2"

	"github.com/masterplotter/loyalty/internal/store"
)

// GET /rewards — katalog publik (reward aktif).
func (a *API) listRewards(c *fiber.Ctx) error {
	rs, err := a.st.ListActiveRewards(c.Context())
	if err != nil {
		return err
	}
	return c.JSON(rs)
}

// POST /rewards/:id/redeem — tukar poin (pembeli yang dikenali).
func (a *API) redeem(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil || id <= 0 {
		return notFound()
	}
	cu := customerFrom(c)
	res, err := a.st.Redeem(c.Context(), cu.ID, id, a.cfg.RedeemRequiresVerified)
	switch {
	case errors.Is(err, store.ErrNotVerified):
		return fiber.NewError(fiber.StatusForbidden, "nomor WhatsApp belum diverifikasi. Hubungi Master Plotter untuk verifikasi.")
	case errors.Is(err, store.ErrInsufficient):
		return conflict("poin kamu belum cukup")
	case errors.Is(err, store.ErrOutOfStock):
		return conflict("stok reward habis")
	case errors.Is(err, store.ErrNotFound):
		return notFound()
	case err != nil:
		return err
	}
	if cu.PhoneVerified {
		a.notifyAsync(cu.Phone, fmt.Sprintf("Penukaran berhasil: %s. Kode: %s. Tunjukkan kode ini ke tim Master Plotter untuk mengambil reward. Sisa poin: %d.", res.Redemption.Reward, res.Redemption.Code, res.Balance))
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"redemption": res.Redemption, "balance": res.Balance})
}

// GET /me/redemptions
func (a *API) myRedemptions(c *fiber.Ctx) error {
	rs, err := a.st.CustomerRedemptions(c.Context(), customerFrom(c).ID)
	if err != nil {
		return err
	}
	return c.JSON(rs)
}
