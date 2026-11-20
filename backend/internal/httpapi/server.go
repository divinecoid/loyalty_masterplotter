// Package httpapi: routing & handler HTTP (Fiber).
package httpapi

import (
	"errors"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/limiter"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/rs/zerolog/log"

	"github.com/masterplotter/loyalty/internal/auth"
	"github.com/masterplotter/loyalty/internal/config"
	"github.com/masterplotter/loyalty/internal/notify"
	"github.com/masterplotter/loyalty/internal/store"
)

const cookieName = "mp_admin"

type API struct {
	cfg    config.Config
	st     *store.Store
	tok    *auth.Tokens
	val    *validator.Validate
	ping   func() error
	sender notify.Sender // nil = OTP/notifikasi WhatsApp nonaktif
}

// Option mengubah dependensi API (dipakai tes).
type Option func(*API)

func WithSender(s notify.Sender) Option { return func(a *API) { a.sender = s } }

func defaultSender(cfg config.Config) notify.Sender {
	switch {
	case cfg.FonnteToken != "":
		return notify.NewFonnte(cfg.FonnteToken)
	case cfg.WAWebhookURL != "":
		return notify.NewWebhook(cfg.WAWebhookURL, cfg.WAWebhookToken)
	case !cfg.Production:
		return notify.LogSender{}
	}
	return nil
}

func New(cfg config.Config, st *store.Store, ping func() error, opts ...Option) *fiber.App {
	a := &API{cfg: cfg, st: st, ping: ping, val: validator.New(),
		tok: auth.NewTokens(cfg.JWTSecret, 12*time.Hour), sender: defaultSender(cfg)}
	for _, o := range opts {
		o(a)
	}

	fcfg := fiber.Config{
		DisableStartupMessage: true,
		BodyLimit:             64 * 1024,
		ReadTimeout:           15 * time.Second,
		WriteTimeout:          60 * time.Second, // PDF stiker bisa besar
		IdleTimeout:           60 * time.Second,
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			code := fiber.StatusInternalServerError
			var fe *fiber.Error
			if errors.As(err, &fe) {
				code = fe.Code
			} else {
				log.Error().Err(err).Str("path", c.Path()).Msg("unhandled")
			}
			msg := "terjadi kesalahan"
			if code < 500 && fe != nil {
				msg = fe.Message
			}
			return c.Status(code).JSON(fiber.Map{"error": msg})
		},
	}
	if cfg.TrustProxy {
		// Di balik reverse proxy (Caddy): rate limiter harus melihat IP klien asli, bukan IP proxy.
		fcfg.ProxyHeader = fiber.HeaderXForwardedFor
	}
	app := fiber.New(fcfg)
	app.Use(recover.New())
	app.Use(securityHeaders(cfg.CookieSecure))
	app.Use(cors.New(cors.Config{AllowOrigins: cfg.CORSOrigins, AllowCredentials: true}))

	api := app.Group("/api/v1")
	api.Get("/health", func(c *fiber.Ctx) error {
		if err := a.ping(); err != nil {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"status": "db_down"})
		}
		return c.JSON(fiber.Map{"status": "ok"})
	})

	// Endpoint publik scan/klaim: rate limit per IP (anti brute-force UUID).
	scan := api.Group("/scan", limiter.New(limiter.Config{Max: 60, Expiration: time.Minute}))
	scan.Get("/:code", a.scanInfo)
	scan.Post("/:code/claim", limiter.New(limiter.Config{Max: 10, Expiration: time.Minute}), a.claim)

	api.Get("/me", a.requireCustomer, a.customerMe)
	api.Get("/me/ledger", a.requireCustomer, a.customerLedger)
	api.Get("/me/redemptions", a.requireCustomer, a.myRedemptions)
	otp := limiter.New(limiter.Config{Max: 10, Expiration: time.Minute})
	api.Post("/me/verify/request", otp, a.requireCustomer, a.otpRequest)
	api.Post("/me/verify/confirm", otp, a.requireCustomer, a.otpConfirm)

	api.Get("/rewards", a.listRewards)
	api.Post("/rewards/:id/redeem", limiter.New(limiter.Config{Max: 10, Expiration: time.Minute}), a.requireCustomer, a.redeem)

	adm := api.Group("/admin")
	adm.Post("/auth/login", limiter.New(limiter.Config{Max: 10, Expiration: time.Minute}), a.login)
	adm.Post("/auth/logout", a.logout)

	secured := adm.Group("", a.requireAdmin)
	secured.Get("/me", a.me)

	secured.Get("/products", a.listProducts)
	secured.Post("/products", a.createProduct)
	secured.Patch("/products/:id", a.updateProduct)

	secured.Get("/batches", a.listBatches)
	secured.Post("/batches", a.createBatch)
	secured.Get("/batches/:id", a.getBatch)
	secured.Get("/batches/:id/sheet.pdf", a.batchSheet)
	secured.Post("/batches/:id/printed", a.markPrinted)
	secured.Post("/batches/:id/activate", a.activateBatch)

	secured.Get("/codes", a.listCodes)
	secured.Post("/codes/:code/void", a.voidCode)

	secured.Get("/summary", a.adminSummary)
	secured.Get("/audit", a.adminAudit)

	secured.Get("/rewards", a.adminListRewards)
	secured.Post("/rewards", a.adminCreateReward)
	secured.Patch("/rewards/:id", a.adminUpdateReward)

	secured.Get("/redemptions", a.adminListRedemptions)
	secured.Post("/redemptions/:id/fulfill", a.adminFulfill)
	secured.Post("/redemptions/:id/cancel", a.adminCancel)

	secured.Get("/customers", a.adminListCustomers)
	secured.Post("/customers/:id/verify", a.adminVerifyCustomer)
	secured.Post("/customers/:id/adjust", a.adminAdjustPoints)
	return app
}

func bad(msg string) error      { return fiber.NewError(fiber.StatusBadRequest, msg) }
func notFound() error           { return fiber.NewError(fiber.StatusNotFound, "tidak ditemukan") }
func conflict(msg string) error { return fiber.NewError(fiber.StatusConflict, msg) }

// mapStoreErr menerjemahkan error store ke HTTP error.
func mapStoreErr(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, store.ErrNotFound):
		return notFound()
	case errors.Is(err, store.ErrConflict):
		return conflict("status tidak mengizinkan operasi ini")
	}
	var pe *pgconn.PgError
	if errors.As(err, &pe) && pe.Code == "23505" {
		return conflict("data sudah ada")
	}
	return err
}

func (a *API) parse(c *fiber.Ctx, dst any) error {
	if err := c.BodyParser(dst); err != nil {
		return bad("body tidak valid")
	}
	if err := a.val.Struct(dst); err != nil {
		return bad("input tidak valid: " + strings.ReplaceAll(err.Error(), "\n", "; "))
	}
	return nil
}

// securityHeaders: header dasar untuk semua respons API. Respons API tidak boleh di-cache (berisi data pribadi / UUID aktif).
func securityHeaders(hsts bool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		c.Set("X-Content-Type-Options", "nosniff")
		c.Set("X-Frame-Options", "DENY")
		c.Set("Referrer-Policy", "no-referrer")
		c.Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
		if hsts {
			c.Set("Strict-Transport-Security", "max-age=31536000")
		}
		if c.Get(fiber.HeaderCacheControl) == "" {
			c.Set(fiber.HeaderCacheControl, "no-store")
		}
		return c.Next()
	}
}
