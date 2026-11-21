package httpapi

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/masterplotter/loyalty/internal/auth"
	"github.com/masterplotter/loyalty/internal/store"
)

type loginReq struct {
	Email    string `json:"email" validate:"required,email,max=254"`
	Password string `json:"password" validate:"required,max=200"`
}

// dummyHash dipakai agar waktu respons login tidak membedakan "email tidak ada" vs "password salah".
var dummyHash, _ = auth.HashPassword("dummy-password-for-timing")

func (a *API) login(c *fiber.Ctx) error {
	var req loginReq
	if err := a.parse(c, &req); err != nil {
		return err
	}
	email := strings.ToLower(strings.TrimSpace(req.Email))
	adm, err := a.st.AdminByEmail(c.Context(), email)
	hash := dummyHash
	if err == nil {
		hash = adm.PasswordHash
	}
	ok := auth.VerifyPassword(req.Password, hash)
	if err != nil || !ok {
		return fiber.NewError(fiber.StatusUnauthorized, "email atau password salah")
	}
	tok, err := a.tok.Issue(adm.ID, adm.Role)
	if err != nil {
		return err
	}
	a.setCookie(c, tok, a.tok.TTL)
	a.st.Audit(c.Context(), adm.ID, "admin.login", fiber.Map{"ip": c.IP()})
	return c.JSON(adm)
}

func (a *API) logout(c *fiber.Ctx) error {
	a.setCookie(c, "", -time.Hour)
	return c.SendStatus(fiber.StatusNoContent)
}

func (a *API) setCookie(c *fiber.Ctx, val string, ttl time.Duration) {
	c.Cookie(&fiber.Cookie{
		Name: cookieName, Value: val, Path: "/", HTTPOnly: true,
		Secure: a.cfg.CookieSecure, SameSite: "Lax",
		Expires: time.Now().Add(ttl), MaxAge: int(ttl.Seconds()),
	})
}

func (a *API) requireAdmin(c *fiber.Ctx) error {
	claims, err := a.tok.Parse(c.Cookies(cookieName))
	if err != nil {
		return fiber.NewError(fiber.StatusUnauthorized, "silakan login")
	}
	c.Locals("admin", claims)
	return c.Next()
}

func adminClaims(c *fiber.Ctx) *auth.Claims { return c.Locals("admin").(*auth.Claims) }

func (a *API) me(c *fiber.Ctx) error {
	adm, err := a.st.AdminByID(c.Context(), adminClaims(c).AdminID)
	if err != nil {
		return fiber.NewError(fiber.StatusUnauthorized, "silakan login")
	}
	return c.JSON(adm)
}

// SeedAdmin membuat admin owner pertama dari env jika tabel admins kosong.
func SeedAdmin(ctx context.Context, st *store.Store, email, password string) (bool, error) {
	if email == "" || password == "" {
		return false, nil
	}
	n, err := st.CountAdmins(ctx)
	if err != nil || n > 0 {
		return false, err
	}
	h, err := auth.HashPassword(password)
	if err != nil {
		return false, err
	}
	_, err = st.CreateAdmin(ctx, strings.ToLower(email), h, "owner")
	return err == nil, err
}

// SyncAdmin membuat atau memperbarui admin owner dari konfigurasi (ADMIN_SYNC=true). created=false berarti password diganti.
func SyncAdmin(ctx context.Context, st *store.Store, email, password string) (created bool, err error) {
	if email == "" || password == "" {
		return false, errors.New("ADMIN_SYNC butuh ADMIN_EMAIL dan ADMIN_PASSWORD")
	}
	h, err := auth.HashPassword(password)
	if err != nil {
		return false, err
	}
	return st.UpsertAdmin(ctx, strings.ToLower(email), h, "owner")
}
