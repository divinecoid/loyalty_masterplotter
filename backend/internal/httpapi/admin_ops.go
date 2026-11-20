package httpapi

import (
	"errors"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/masterplotter/loyalty/internal/store"
)

func pathID(c *fiber.Ctx) (int64, error) {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil || id <= 0 {
		return 0, notFound()
	}
	return id, nil
}

func paging(c *fiber.Ctx) (limit, offset int) {
	return min(max(c.QueryInt("limit", 50), 1), 200), max(c.QueryInt("offset", 0), 0)
}

// ---------- Reward ----------

func (a *API) adminListRewards(c *fiber.Ctx) error {
	rs, err := a.st.ListAllRewards(c.Context())
	if err != nil {
		return err
	}
	return c.JSON(rs)
}

type rewardReq struct {
	Name     string  `json:"name" validate:"required,max=200"`
	Cost     int     `json:"cost_points" validate:"required,gte=1,lte=10000000"`
	Stock    int     `json:"stock" validate:"gte=0,lte=1000000"`
	ImageURL *string `json:"image_url" validate:"omitempty,url,max=500"`
}

func (a *API) adminCreateReward(c *fiber.Ctx) error {
	var req rewardReq
	if err := a.parse(c, &req); err != nil {
		return err
	}
	r, err := a.st.CreateReward(c.Context(), strings.TrimSpace(req.Name), req.Cost, req.Stock, req.ImageURL)
	if err != nil {
		return mapStoreErr(err)
	}
	a.st.Audit(c.Context(), adminClaims(c).AdminID, "reward.create", r)
	return c.Status(fiber.StatusCreated).JSON(r)
}

type rewardPatch struct {
	Name   *string `json:"name" validate:"omitempty,max=200"`
	Cost   *int    `json:"cost_points" validate:"omitempty,gte=1,lte=10000000"`
	Stock  *int    `json:"stock" validate:"omitempty,gte=0,lte=1000000"`
	Active *bool   `json:"active"`
}

func (a *API) adminUpdateReward(c *fiber.Ctx) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	var req rewardPatch
	if err := a.parse(c, &req); err != nil {
		return err
	}
	r, err := a.st.UpdateReward(c.Context(), id, req.Name, req.Cost, req.Stock, req.Active)
	if err != nil {
		return mapStoreErr(err)
	}
	a.st.Audit(c.Context(), adminClaims(c).AdminID, "reward.update", r)
	return c.JSON(r)
}

// ---------- Penukaran ----------

func (a *API) adminListRedemptions(c *fiber.Ctx) error {
	status := c.Query("status")
	switch status {
	case "", "pending", "fulfilled", "cancelled":
	default:
		return bad("status tidak valid")
	}
	limit, offset := paging(c)
	rs, err := a.st.ListRedemptions(c.Context(), status, c.Query("q"), limit, offset)
	if err != nil {
		return err
	}
	return c.JSON(rs)
}

func (a *API) adminFulfill(c *fiber.Ctx) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	if err := a.st.FulfillRedemption(c.Context(), id); err != nil {
		return mapStoreErr(err)
	}
	a.st.Audit(c.Context(), adminClaims(c).AdminID, "redemption.fulfill", fiber.Map{"id": id})
	return c.SendStatus(fiber.StatusNoContent)
}

func (a *API) adminCancel(c *fiber.Ctx) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	if err := a.st.CancelRedemption(c.Context(), id); err != nil {
		return mapStoreErr(err)
	}
	a.st.Audit(c.Context(), adminClaims(c).AdminID, "redemption.cancel", fiber.Map{"id": id})
	return c.SendStatus(fiber.StatusNoContent)
}

// ---------- Pelanggan ----------

func (a *API) adminListCustomers(c *fiber.Ctx) error {
	limit, offset := paging(c)
	cs, err := a.st.ListCustomers(c.Context(), c.Query("q"), limit, offset)
	if err != nil {
		return err
	}
	return c.JSON(cs)
}

type verifyReq struct {
	Verified bool `json:"verified"`
}

func (a *API) adminVerifyCustomer(c *fiber.Ctx) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	var req verifyReq
	if err := a.parse(c, &req); err != nil {
		return err
	}
	if err := a.st.SetPhoneVerified(c.Context(), id, req.Verified); err != nil {
		return mapStoreErr(err)
	}
	a.st.Audit(c.Context(), adminClaims(c).AdminID, "customer.verify", fiber.Map{"id": id, "verified": req.Verified})
	return c.SendStatus(fiber.StatusNoContent)
}

type adjustReq struct {
	Delta int `json:"delta" validate:"required,gte=-1000000,lte=1000000"`
}

func (a *API) adminAdjustPoints(c *fiber.Ctx) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	var req adjustReq
	if err := a.parse(c, &req); err != nil {
		return err
	}
	bal, err := a.st.AdjustPoints(c.Context(), id, req.Delta)
	if errors.Is(err, store.ErrBadDelta) {
		return conflict("saldo tidak boleh negatif")
	}
	if err != nil {
		return mapStoreErr(err)
	}
	a.st.Audit(c.Context(), adminClaims(c).AdminID, "customer.adjust", fiber.Map{"id": id, "delta": req.Delta})
	return c.JSON(fiber.Map{"points": bal})
}

// ---------- Ringkasan & audit ----------

func (a *API) adminSummary(c *fiber.Ctx) error {
	m, err := a.st.Summary(c.Context())
	if err != nil {
		return err
	}
	return c.JSON(m)
}

func (a *API) adminAudit(c *fiber.Ctx) error {
	limit, offset := paging(c)
	es, err := a.st.ListAudit(c.Context(), limit, offset)
	if err != nil {
		return err
	}
	return c.JSON(es)
}
