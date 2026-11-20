package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/masterplotter/loyalty/internal/config"
)

func TestAdminOps(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	cfg := config.Config{JWTSecret: "test-secret-test-secret-test-secret", CORSOrigins: "http://localhost:5173", RedeemRequiresVerified: true}
	app := New(cfg, e.st, func() error { return e.pool.Ping(ctx) })

	email := "ops-" + uuid.NewString()[:8] + "@test.local"
	if _, err := SeedAdminForce(ctx, e.st, email, "pass-12345"); err != nil {
		t.Fatal(err)
	}
	anon := &client{t: t, app: app}
	anon.do("GET", "/api/v1/admin/summary", nil, 401)
	anon.do("POST", "/api/v1/admin/rewards", map[string]any{"name": "x", "cost_points": 1}, 401)

	adm := &client{t: t, app: app}
	adm.do("POST", "/api/v1/admin/auth/login", map[string]any{"email": email, "password": "pass-12345"}, 200)

	// reward CRUD
	adm.do("POST", "/api/v1/admin/rewards", map[string]any{"name": "", "cost_points": 10}, 400)
	rw := adm.do("POST", "/api/v1/admin/rewards", map[string]any{"name": "Topi", "cost_points": 20, "stock": 3}, 201)
	rid := int64(rw["id"].(float64))
	adm.do("PATCH", fmt.Sprintf("/api/v1/admin/rewards/%d", rid), map[string]any{"stock": 0}, 200)
	adm.do("PATCH", "/api/v1/admin/rewards/999999999", map[string]any{"stock": 1}, 404)
	adm.do("PATCH", fmt.Sprintf("/api/v1/admin/rewards/%d", rid), map[string]any{"stock": 3}, 200)

	// pelanggan: verifikasi lewat admin lalu tukar
	cid, tok := e.newCustomer(t, 50, false)
	cu := &client{t: t, app: app, cookie: &http.Cookie{Name: deviceCookie, Value: tok}}
	cu.do("POST", fmt.Sprintf("/api/v1/rewards/%d/redeem", rid), nil, 403)
	adm.do("POST", fmt.Sprintf("/api/v1/admin/customers/%d/verify", cid), map[string]any{"verified": true}, 204)
	red := cu.do("POST", fmt.Sprintf("/api/v1/rewards/%d/redeem", rid), nil, 201)
	redID := int64(red["redemption"].(map[string]any)["id"].(float64))
	code := red["redemption"].(map[string]any)["code"].(string)

	// cari lewat kode
	list := func(q string) int {
		var n int
		resp, err := e.st.ListRedemptions(ctx, "", q, 10, 0)
		if err != nil {
			t.Fatal(err)
		}
		n = len(resp)
		return n
	}
	if list(code) != 1 {
		t.Fatalf("pencarian kode gagal")
	}
	adm.do("GET", "/api/v1/admin/redemptions?status=pending&q="+code, nil, 200)
	adm.do("GET", "/api/v1/admin/redemptions?status=bogus", nil, 400)

	// batalkan: poin & stok kembali, ledger tercatat
	adm.do("POST", fmt.Sprintf("/api/v1/admin/redemptions/%d/cancel", redID), nil, 204)
	var bal, stock int
	e.pool.QueryRow(ctx, `SELECT points_balance FROM customers WHERE id=$1`, cid).Scan(&bal)
	e.pool.QueryRow(ctx, `SELECT stock FROM rewards WHERE id=$1`, rid).Scan(&stock)
	if bal != 50 || stock != 3 {
		t.Fatalf("setelah batal: saldo=%d stok=%d, want 50/3", bal, stock)
	}
	var sum int
	e.pool.QueryRow(ctx, `SELECT COALESCE(sum(delta),0) FROM point_ledger WHERE customer_id=$1`, cid).Scan(&sum)
	if sum != 0 { // fixture memberi saldo 50 tanpa ledger: -20 (redeem) +20 (refund) = 0
		t.Fatalf("ledger sum=%d want 0", sum)
	}
	adm.do("POST", fmt.Sprintf("/api/v1/admin/redemptions/%d/cancel", redID), nil, 409)
	adm.do("POST", fmt.Sprintf("/api/v1/admin/redemptions/%d/fulfill", redID), nil, 409)

	// tukar lagi lalu fulfill; setelah fulfilled tidak bisa dibatalkan
	red2 := cu.do("POST", fmt.Sprintf("/api/v1/rewards/%d/redeem", rid), nil, 201)
	id2 := int64(red2["redemption"].(map[string]any)["id"].(float64))
	adm.do("POST", fmt.Sprintf("/api/v1/admin/redemptions/%d/fulfill", id2), nil, 204)
	adm.do("POST", fmt.Sprintf("/api/v1/admin/redemptions/%d/fulfill", id2), nil, 409)
	adm.do("POST", fmt.Sprintf("/api/v1/admin/redemptions/%d/cancel", id2), nil, 409)
	adm.do("POST", "/api/v1/admin/redemptions/999999999/cancel", nil, 404)

	// adjust poin
	if m := adm.do("POST", fmt.Sprintf("/api/v1/admin/customers/%d/adjust", cid), map[string]any{"delta": 5}, 200); m["points"].(float64) != 35 {
		t.Fatalf("adjust: %v", m)
	}
	adm.do("POST", fmt.Sprintf("/api/v1/admin/customers/%d/adjust", cid), map[string]any{"delta": -1000}, 409)
	adm.do("POST", fmt.Sprintf("/api/v1/admin/customers/%d/adjust", cid), map[string]any{"delta": 0}, 400)

	adm.do("GET", "/api/v1/admin/customers?q=%2B62877", nil, 200)
	adm.do("GET", "/api/v1/admin/summary", nil, 200)
	if m := adm.do("GET", "/api/v1/admin/rewards", nil, 200); m == nil {
		t.Fatal("rewards")
	}
	adm.do("GET", "/api/v1/admin/audit", nil, 200)
}

// Batalkan vs. fulfill berbarengan: tepat satu menang, dan saldo konsisten.
func TestCancelFulfillRace(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	cid, _ := e.newCustomer(t, 20, true)
	rid := e.newReward(t, 20, 1)
	res, err := e.st.Redeem(ctx, cid, rid, true)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	var mu sync.Mutex
	wins := 0
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			var err error
			if i%2 == 0 {
				err = e.st.CancelRedemption(ctx, res.Redemption.ID)
			} else {
				err = e.st.FulfillRedemption(ctx, res.Redemption.ID)
			}
			if err == nil {
				mu.Lock()
				wins++
				mu.Unlock()
			}
		}(i)
	}
	wg.Wait()
	if wins != 1 {
		t.Fatalf("wins=%d want 1", wins)
	}
	var status string
	var bal, stock int
	e.pool.QueryRow(ctx, `SELECT status FROM redemptions WHERE id=$1`, res.Redemption.ID).Scan(&status)
	e.pool.QueryRow(ctx, `SELECT points_balance FROM customers WHERE id=$1`, cid).Scan(&bal)
	e.pool.QueryRow(ctx, `SELECT stock FROM rewards WHERE id=$1`, rid).Scan(&stock)
	if (status == "cancelled" && (bal != 20 || stock != 1)) || (status == "fulfilled" && (bal != 0 || stock != 0)) {
		t.Fatalf("tidak konsisten: status=%s bal=%d stock=%d", status, bal, stock)
	}
}
