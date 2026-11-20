package httpapi

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/masterplotter/loyalty/internal/qr"
	"github.com/masterplotter/loyalty/internal/sticker"
)

const maxBatchQty = 10000

// ---------- Produk ----------

func (a *API) listProducts(c *fiber.Ctx) error {
	ps, err := a.st.ListProducts(c.Context())
	if err != nil {
		return err
	}
	return c.JSON(ps)
}

type productReq struct {
	SKU    string `json:"sku" validate:"required,max=64"`
	Name   string `json:"name" validate:"required,max=200"`
	Points int    `json:"points_per_unit" validate:"gte=0,lte=1000000"`
}

func (a *API) createProduct(c *fiber.Ctx) error {
	var req productReq
	if err := a.parse(c, &req); err != nil {
		return err
	}
	p, err := a.st.CreateProduct(c.Context(), strings.TrimSpace(req.SKU), strings.TrimSpace(req.Name), req.Points)
	if err != nil {
		return mapStoreErr(err)
	}
	a.st.Audit(c.Context(), adminClaims(c).AdminID, "product.create", p)
	return c.Status(fiber.StatusCreated).JSON(p)
}

type productPatch struct {
	Name   *string `json:"name" validate:"omitempty,max=200"`
	Points *int    `json:"points_per_unit" validate:"omitempty,gte=0,lte=1000000"`
	Active *bool   `json:"active"`
}

func (a *API) updateProduct(c *fiber.Ctx) error {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return notFound()
	}
	var req productPatch
	if err := a.parse(c, &req); err != nil {
		return err
	}
	p, err := a.st.UpdateProduct(c.Context(), id, req.Name, req.Points, req.Active)
	if err != nil {
		return mapStoreErr(err)
	}
	a.st.Audit(c.Context(), adminClaims(c).AdminID, "product.update", p)
	return c.JSON(p)
}

// ---------- Batch ----------

func (a *API) listBatches(c *fiber.Ctx) error {
	bs, err := a.st.ListBatches(c.Context())
	if err != nil {
		return err
	}
	return c.JSON(bs)
}

type batchReq struct {
	ProductID int64  `json:"product_id" validate:"required,gt=0"`
	Qty       int    `json:"qty" validate:"required,gte=1,lte=10000"`
	Label     string `json:"label" validate:"max=100"`
}

func (a *API) createBatch(c *fiber.Ctx) error {
	var req batchReq
	if err := a.parse(c, &req); err != nil {
		return err
	}
	id, err := a.st.CreateBatch(c.Context(), req.ProductID, strings.TrimSpace(req.Label), req.Qty, adminClaims(c).AdminID)
	if err != nil {
		return mapStoreErr(err)
	}
	a.st.Audit(c.Context(), adminClaims(c).AdminID, "batch.create", fiber.Map{"batch_id": id, "qty": req.Qty})
	b, err := a.st.BatchByID(c.Context(), id)
	if err != nil {
		return mapStoreErr(err)
	}
	return c.Status(fiber.StatusCreated).JSON(b)
}

func batchID(c *fiber.Ctx) (int64, error) {
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil || id <= 0 {
		return 0, notFound()
	}
	return id, nil
}

func (a *API) getBatch(c *fiber.Ctx) error {
	id, err := batchID(c)
	if err != nil {
		return err
	}
	b, err := a.st.BatchByID(c.Context(), id)
	if err != nil {
		return mapStoreErr(err)
	}
	return c.JSON(b)
}

// batchSheet: PDF stiker. Berisi UUID aktif -> hanya admin, tidak di-cache, dan dicatat di audit log.
func (a *API) batchSheet(c *fiber.Ctx) error {
	id, err := batchID(c)
	if err != nil {
		return err
	}
	b, err := a.st.BatchByID(c.Context(), id)
	if err != nil {
		return mapStoreErr(err)
	}
	cell := 30.0
	if v := c.Query("cell"); v != "" {
		if cell, err = strconv.ParseFloat(v, 64); err != nil {
			return bad("cell tidak valid")
		}
	}
	codes, err := a.st.BatchCodes(c.Context(), id)
	if err != nil {
		return err
	}
	items := make([]sticker.Item, len(codes))
	for i, code := range codes {
		items[i] = sticker.Item{Content: qr.URL(a.cfg.PublicURL, code), Label: fmt.Sprintf("B%d-%04d", id, i+1)}
	}
	var buf bytes.Buffer
	if err := sticker.Render(&buf, items, sticker.Options{
		CellMM: cell, Title: truncate(b.ProductName, 28), Guides: c.Query("guides", "1") == "1",
	}); err != nil {
		return bad(err.Error())
	}
	a.st.Audit(c.Context(), adminClaims(c).AdminID, "batch.sheet", fiber.Map{"batch_id": id, "count": len(codes)})
	c.Set(fiber.HeaderContentType, "application/pdf")
	c.Set(fiber.HeaderCacheControl, "no-store")
	c.Set(fiber.HeaderContentDisposition, fmt.Sprintf(`attachment; filename="stiker-batch-%d.pdf"`, id))
	return c.Send(buf.Bytes())
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

func (a *API) markPrinted(c *fiber.Ctx) error {
	id, err := batchID(c)
	if err != nil {
		return err
	}
	if err := a.st.MarkPrinted(c.Context(), id); err != nil {
		return mapStoreErr(err)
	}
	a.st.Audit(c.Context(), adminClaims(c).AdminID, "batch.printed", fiber.Map{"batch_id": id})
	return c.SendStatus(fiber.StatusNoContent)
}

func (a *API) activateBatch(c *fiber.Ctx) error {
	id, err := batchID(c)
	if err != nil {
		return err
	}
	if err := a.st.ActivateBatch(c.Context(), id); err != nil {
		return mapStoreErr(err)
	}
	a.st.Audit(c.Context(), adminClaims(c).AdminID, "batch.activate", fiber.Map{"batch_id": id})
	return c.SendStatus(fiber.StatusNoContent)
}

// ---------- Kode ----------

func (a *API) listCodes(c *fiber.Ctx) error {
	status := c.Query("status")
	switch status {
	case "", "generated", "active", "claimed", "void":
	default:
		return bad("status tidak valid")
	}
	bid, _ := strconv.ParseInt(c.Query("batch_id", "0"), 10, 64)
	limit := min(max(c.QueryInt("limit", 50), 1), 200)
	offset := max(c.QueryInt("offset", 0), 0)
	cs, err := a.st.ListCodes(c.Context(), status, bid, limit, offset)
	if err != nil {
		return err
	}
	return c.JSON(cs)
}

func (a *API) voidCode(c *fiber.Ctx) error {
	code, err := uuid.Parse(c.Params("code"))
	if err != nil {
		return notFound()
	}
	if err := a.st.VoidCode(c.Context(), code); err != nil {
		return mapStoreErr(err)
	}
	a.st.Audit(c.Context(), adminClaims(c).AdminID, "code.void", fiber.Map{"code": code})
	return c.SendStatus(fiber.StatusNoContent)
}
