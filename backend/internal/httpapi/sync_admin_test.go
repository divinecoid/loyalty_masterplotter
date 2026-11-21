package httpapi

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/masterplotter/loyalty/internal/auth"
)

func TestSyncAdmin(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	email := "Sync-" + uuid.NewString()[:8] + "@Test.Local"

	if _, err := SyncAdmin(ctx, e.st, "", "x"); err == nil {
		t.Fatal("email kosong harus ditolak")
	}
	created, err := SyncAdmin(ctx, e.st, email, "first-password-1")
	if err != nil || !created {
		t.Fatalf("buat: created=%v err=%v", created, err)
	}
	created, err = SyncAdmin(ctx, e.st, email, "second-password-2")
	if err != nil || created {
		t.Fatalf("update: created=%v err=%v", created, err)
	}
	a, err := e.st.AdminByEmail(ctx, strings.ToLower(email))
	if err != nil {
		t.Fatal(err)
	}
	if a.Role != "owner" || !auth.VerifyPassword("second-password-2", a.PasswordHash) || auth.VerifyPassword("first-password-1", a.PasswordHash) {
		t.Fatal("password tidak terganti dengan benar")
	}
}
