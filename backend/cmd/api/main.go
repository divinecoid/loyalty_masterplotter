package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/rs/zerolog/log"

	"github.com/masterplotter/loyalty/internal/config"
	"github.com/masterplotter/loyalty/internal/db"
	"github.com/masterplotter/loyalty/internal/httpapi"
	"github.com/masterplotter/loyalty/internal/store"
)

func main() {
	cfg := config.Load()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := cfg.Validate(); err != nil {
		log.Fatal().Err(err).Msg("konfigurasi produksi tidak aman")
	}
	if !cfg.Production {
		log.Warn().Msg("PRODUCTION=false: validasi konfigurasi ketat dimatikan (hanya untuk dev)")
	}

	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal().Err(err).Msg("connect database")
	}
	defer pool.Close()

	if cfg.AutoMigrate {
		if err := db.Migrate(pool); err != nil {
			log.Fatal().Err(err).Msg("migrate")
		}
	}

	st := store.New(pool)
	if cfg.AdminSync {
		created, err := httpapi.SyncAdmin(ctx, st, cfg.AdminEmail, cfg.AdminPassword)
		if err != nil {
			log.Fatal().Err(err).Msg("sync admin")
		}
		log.Warn().Str("email", cfg.AdminEmail).Bool("created", created).Msg("ADMIN_SYNC aktif: admin owner disinkronkan dari env — matikan ADMIN_SYNC dan hapus ADMIN_PASSWORD setelah ini")
	} else if created, err := httpapi.SeedAdmin(ctx, st, cfg.AdminEmail, cfg.AdminPassword); err != nil {
		log.Fatal().Err(err).Msg("seed admin")
	} else if created {
		log.Info().Str("email", cfg.AdminEmail).Msg("admin owner pertama dibuat")
	}

	app := httpapi.New(cfg, st, func() error { return pool.Ping(ctx) })
	go func() {
		<-ctx.Done()
		_ = app.Shutdown()
	}()
	log.Info().Str("port", cfg.Port).Msg("api listening")
	if err := app.Listen(":" + cfg.Port); err != nil {
		log.Fatal().Err(err).Msg("listen")
	}
}
