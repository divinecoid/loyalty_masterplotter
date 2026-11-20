package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/google/uuid"

	"github.com/masterplotter/loyalty/internal/config"
	"github.com/masterplotter/loyalty/internal/db"
	"github.com/masterplotter/loyalty/internal/store"
)

// Integration test: jalankan dengan TEST_DATABASE_URL=postgres://... (non-destruktif: pakai SKU/email unik).
func TestAdminFlow(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL tidak di-set")
	}
	ctx := context.Background()
	pool, err := db.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err := db.Migrate(pool); err != nil {
		t.Fatal(err)
	}
	st := store.New(pool)
	email := "t-" + uuid.NewString()[:8] + "@test.local"
	if _, err := SeedAdminForce(ctx, st, email, "pass-12345"); err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{JWTSecret: "test-secret-test-secret-test-secret", PublicURL: "https://loyalty.test", CORSOrigins: "http://localhost:5173"}
	app := New(cfg, st, func() error { return pool.Ping(ctx) })

	var cookie *http.Cookie
	do := func(method, path string, body any, wantStatus int) []byte {
		t.Helper()
		var rd io.Reader
		if body != nil {
			b, _ := json.Marshal(body)
			rd = bytes.NewReader(b)
		}
		req := httptest.NewRequest(method, path, rd)
		req.Header.Set("Content-Type", "application/json")
		if cookie != nil {
			req.AddCookie(cookie)
		}
		resp, err := app.Test(req, 20000)
		if err != nil {
			t.Fatal(err)
		}
		out, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != wantStatus {
			t.Fatalf("%s %s: want %d got %d: %s", method, path, wantStatus, resp.StatusCode, out)
		}
		for _, c := range resp.Cookies() {
			if c.Name == cookieName {
				cookie = c
			}
		}
		return out
	}

	do("GET", "/api/v1/admin/products", nil, 401) // tanpa login
	do("POST", "/api/v1/admin/auth/login", map[string]string{"email": email, "password": "salah"}, 401)
	do("POST", "/api/v1/admin/auth/login", map[string]string{"email": email, "password": "pass-12345"}, 200)

	sku := "SKU-" + uuid.NewString()[:8]
	var prod store.Product
	_ = json.Unmarshal(do("POST", "/api/v1/admin/products", map[string]any{"sku": sku, "name": "Tinta Epson 664", "points_per_unit": 10}, 201), &prod)
	do("POST", "/api/v1/admin/products", map[string]any{"sku": sku, "name": "dup", "points_per_unit": 1}, 409)

	do("POST", "/api/v1/admin/batches", map[string]any{"product_id": prod.ID, "qty": 0}, 400)
	do("POST", "/api/v1/admin/batches", map[string]any{"product_id": prod.ID, "qty": 10001}, 400)
	var b store.Batch
	_ = json.Unmarshal(do("POST", "/api/v1/admin/batches", map[string]any{"product_id": prod.ID, "qty": 60, "label": "uji"}, 201), &b)
	if b.Qty != 60 || b.Counts["generated"] != 60 {
		t.Fatalf("batch: %+v", b)
	}

	pdf := do("GET", "/api/v1/admin/batches/"+itoa(b.ID)+"/sheet.pdf", nil, 200)
	if !bytes.HasPrefix(pdf, []byte("%PDF-")) {
		t.Fatal("sheet bukan PDF")
	}

	do("POST", "/api/v1/admin/batches/"+itoa(b.ID)+"/activate", nil, 409) // belum printed
	do("POST", "/api/v1/admin/batches/"+itoa(b.ID)+"/printed", nil, 204)
	do("POST", "/api/v1/admin/batches/"+itoa(b.ID)+"/printed", nil, 409) // sudah
	do("POST", "/api/v1/admin/batches/"+itoa(b.ID)+"/activate", nil, 204)

	_ = json.Unmarshal(do("GET", "/api/v1/admin/batches/"+itoa(b.ID), nil, 200), &b)
	if b.Status != "active" || b.Counts["active"] != 60 {
		t.Fatalf("setelah aktivasi: %+v", b)
	}

	var codes []store.Code
	_ = json.Unmarshal(do("GET", "/api/v1/admin/codes?status=active&batch_id="+itoa(b.ID)+"&limit=5", nil, 200), &codes)
	if len(codes) != 5 || codes[0].Code.Version() != 4 || codes[0].Points != 10 {
		t.Fatalf("codes: %+v", codes)
	}
	do("POST", "/api/v1/admin/codes/"+codes[0].Code.String()+"/void", nil, 204)
	do("POST", "/api/v1/admin/codes/"+codes[0].Code.String()+"/void", nil, 409)
	do("GET", "/api/v1/admin/codes?status=hack", nil, 400)
}

func itoa(n int64) string { b, _ := json.Marshal(n); return string(b) }

// SeedAdminForce: helper tes — buat admin tanpa syarat tabel kosong.
func SeedAdminForce(ctx context.Context, st *store.Store, email, pw string) (store.Admin, error) {
	h, err := hashForTest(pw)
	if err != nil {
		return store.Admin{}, err
	}
	return st.CreateAdmin(ctx, email, h, "owner")
}
