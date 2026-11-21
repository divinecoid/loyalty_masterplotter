package config

import (
	"errors"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Port        string
	DatabaseURL string
	PublicURL   string // base URL yang ditanam di QR, mis. https://loyalty.example.com
	CORSOrigins string

	JWTSecret    string
	CookieSecure bool
	AutoMigrate  bool

	// Production: aktifkan validasi ketat saat start. TrustProxy: percaya X-Forwarded-For (hanya jika API tidak terekspos langsung).
	Production bool

	// Gateway WhatsApp (OTP & notifikasi). Kosong + non-produksi = pesan hanya ditulis ke log; kosong + produksi = OTP nonaktif.
	FonnteToken    string // prioritas pertama jika diisi
	WAWebhookURL   string
	WAWebhookToken string
	TrustProxy     bool

	// Tukar reward hanya untuk nomor yang sudah diverifikasi (OTP WA / konfirmasi staf).
	RedeemRequiresVerified bool

	// Bootstrap admin pertama (hanya dipakai jika tabel admins masih kosong)
	AdminEmail    string
	AdminPassword string
	// AdminSync: saat start, buat/perbarui admin owner dari ADMIN_EMAIL+ADMIN_PASSWORD (untuk reset). Matikan lagi setelah dipakai.
	AdminSync bool
}

func Load() Config {
	return Config{
		Port:           env("PORT", "8080"),
		DatabaseURL:    env("DATABASE_URL", "postgres://loyalty:loyalty@localhost:5433/loyalty?sslmode=disable"),
		PublicURL:      env("PUBLIC_URL", "http://localhost:5173"),
		CORSOrigins:    env("CORS_ORIGINS", "http://localhost:5173"),
		JWTSecret:      env("JWT_SECRET", "dev-only-change-me-dev-only-change-me"),
		CookieSecure:   envBool("COOKIE_SECURE", false),
		AutoMigrate:    envBool("AUTO_MIGRATE", true),
		Production:     envBool("PRODUCTION", false),
		FonnteToken:    os.Getenv("FONNTE_TOKEN"),
		WAWebhookURL:   os.Getenv("WA_WEBHOOK_URL"),
		WAWebhookToken: os.Getenv("WA_WEBHOOK_TOKEN"),
		TrustProxy:     envBool("TRUST_PROXY", false),

		RedeemRequiresVerified: envBool("REDEEM_REQUIRES_VERIFIED", true),
		AdminEmail:             os.Getenv("ADMIN_EMAIL"),
		AdminPassword:          os.Getenv("ADMIN_PASSWORD"),
		AdminSync:              envBool("ADMIN_SYNC", false),
	}
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func envBool(k string, def bool) bool {
	if v := os.Getenv(k); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return def
}

const devSecret = "dev-only-change-me-dev-only-change-me"

// Validate menolak konfigurasi tidak aman saat PRODUCTION=true.
func (c Config) Validate() error {
	if !c.Production {
		return nil
	}
	var errs []error
	if c.JWTSecret == devSecret || len(c.JWTSecret) < 32 {
		errs = append(errs, errors.New("JWT_SECRET wajib diisi acak, minimal 32 karakter"))
	}
	if !c.CookieSecure {
		errs = append(errs, errors.New("COOKIE_SECURE harus true (HTTPS)"))
	}
	if !strings.HasPrefix(c.PublicURL, "https://") {
		errs = append(errs, errors.New("PUBLIC_URL harus https://"))
	}
	if strings.Contains(c.CORSOrigins, "*") {
		errs = append(errs, errors.New("CORS_ORIGINS tidak boleh wildcard"))
	}
	if strings.Contains(c.DatabaseURL, "loyalty:loyalty@") {
		errs = append(errs, errors.New("DATABASE_URL memakai kredensial default"))
	}
	if c.AdminPassword != "" && len(c.AdminPassword) < 12 {
		errs = append(errs, errors.New("ADMIN_PASSWORD minimal 12 karakter"))
	}
	return errors.Join(errs...)
}
