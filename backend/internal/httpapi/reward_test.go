package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/masterplotter/loyalty/internal/config"
	"github.com/masterplotter/loyalty/internal/store"
)

func (e env) newReward(t *testing.T, cost, stock int) int64 {
	t.Helper()
	var id int64
	err := e.pool.QueryRow(context.Background(),
		`INSERT INTO rewards(name,cost_points,stock) VALUES($1,$2,$3) RETURNING id`,
		"Reward "+uuid.NewString()[:6], cost, stock).Scan(&id)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

// newCustomer membuat pelanggan + perangkat; mengembalikan id dan token cookie.
func (e env) newCustomer(t *testing.T, points int, verified bool) (int64, string) {
	t.Helper()
	ctx := context.Background()
	var id int64
	phone := fmt.Sprintf("+62877%08d", uuid.New().ID()%100000000)
	if err := e.pool.QueryRow(ctx,
		`INSERT INTO customers(name,phone_e164,points_balance,phone_verified) VALUES('Uji',$1,$2,$3) RETURNING id`,
		phone, points, verified).Scan(&id); err != nil {
		t.Fatal(err)
	}
	tok, hash, _ := newDeviceToken()
	if _, err := e.pool.Exec(ctx, `INSERT INTO devices(customer_id,token_hash) VALUES($1,$2)`, id, hash); err != nil {
		t.Fatal(err)
	}
	return id, tok
}

func TestRedeemHTTP(t *testing.T) {
	e := setup(t)
	cfg := config.Config{JWTSecret: "test-secret-test-secret-test-secret", CORSOrigins: "http://localhost:5173", RedeemRequiresVerified: true}
	app := New(cfg, e.st, func() error { return e.pool.Ping(context.Background()) })
	cid, tok := e.newCustomer(t, 50, false)
	cheap := e.newReward(t, 20, 5)
	dear := e.newReward(t, 100, 5)
	empty := e.newReward(t, 10, 0)

	anon := &client{t: t, app: app}
	anon.do("POST", fmt.Sprintf("/api/v1/rewards/%d/redeem", cheap), nil, 401)

	c := &client{t: t, app: app}
	c.cookie = &http.Cookie{Name: deviceCookie, Value: tok}
	redeem := func(id int64, want int) map[string]any {
		return c.do("POST", fmt.Sprintf("/api/v1/rewards/%d/redeem", id), nil, want)
	}

	// katalog publik memuat reward aktif
	body := anon.do("GET", "/api/v1/rewards", nil, 200)
	_ = body

	redeem(cheap, 403) // belum diverifikasi
	if _, err := e.pool.Exec(context.Background(), `UPDATE customers SET phone_verified=true WHERE id=$1`, cid); err != nil {
		t.Fatal(err)
	}
	redeem(dear, 409)     // poin kurang
	redeem(empty, 409)    // stok habis
	redeem(99999999, 404) // tidak ada

	m := redeem(cheap, 201)
	r := m["redemption"].(map[string]any)
	if m["balance"].(float64) != 30 || r["status"] != "pending" || r["cost"].(float64) != 20 {
		t.Fatalf("hasil: %v", m)
	}
	if !regexp.MustCompile(`^[2-9A-HJ-NP-Z]{4}-[2-9A-HJ-NP-Z]{4}$`).MatchString(r["code"].(string)) {
		t.Fatalf("format kode: %v", r["code"])
	}
	if me := c.do("GET", "/api/v1/me", nil, 200); me["points"].(float64) != 30 {
		t.Fatalf("me: %v", me)
	}
	var stock int
	_ = e.pool.QueryRow(context.Background(), `SELECT stock FROM rewards WHERE id=$1`, cheap).Scan(&stock)
	var delta int
	_ = e.pool.QueryRow(context.Background(),
		`SELECT delta FROM point_ledger WHERE customer_id=$1 AND reason='redeem'`, cid).Scan(&delta)
	if stock != 4 || delta != -20 {
		t.Fatalf("stock=%d delta=%d", stock, delta)
	}
}

func TestRedeemGateOffAllowsUnverified(t *testing.T) {
	e := setup(t)
	cid, _ := e.newCustomer(t, 50, false)
	rid := e.newReward(t, 10, 1)
	if _, err := e.st.Redeem(context.Background(), cid, rid, true); err != store.ErrNotVerified {
		t.Fatalf("want ErrNotVerified, got %v", err)
	}
	if _, err := e.st.Redeem(context.Background(), cid, rid, false); err != nil {
		t.Fatal(err)
	}
}

// Stok 1, 10 pembeli bersamaan -> tepat satu berhasil.
func TestRedeemRaceStock(t *testing.T) {
	e := setup(t)
	rid := e.newReward(t, 10, 1)
	const n = 10
	var wg sync.WaitGroup
	errs := make([]error, n)
	for i := 0; i < n; i++ {
		cid, _ := e.newCustomer(t, 50, true)
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, errs[i] = e.st.Redeem(context.Background(), cid, rid, true)
		}()
	}
	wg.Wait()
	ok, oos := 0, 0
	for _, err := range errs {
		switch err {
		case nil:
			ok++
		case store.ErrOutOfStock:
			oos++
		default:
			t.Fatalf("error tak terduga: %v", err)
		}
	}
	var stock int
	_ = e.pool.QueryRow(context.Background(), `SELECT stock FROM rewards WHERE id=$1`, rid).Scan(&stock)
	if ok != 1 || oos != n-1 || stock != 0 {
		t.Fatalf("ok=%d oos=%d stock=%d", ok, oos, stock)
	}
}

// Satu pembeli 30 poin menukar reward 20 poin 6x bersamaan -> hanya satu berhasil, saldo tidak negatif.
func TestRedeemRaceBalance(t *testing.T) {
	e := setup(t)
	rid := e.newReward(t, 20, 100)
	cid, _ := e.newCustomer(t, 30, true)
	const n = 6
	var wg sync.WaitGroup
	errs := make([]error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, errs[i] = e.st.Redeem(context.Background(), cid, rid, true)
		}()
	}
	wg.Wait()
	ok, insuf := 0, 0
	for _, err := range errs {
		switch err {
		case nil:
			ok++
		case store.ErrInsufficient:
			insuf++
		default:
			t.Fatalf("error tak terduga: %v", err)
		}
	}
	var bal, stock int
	_ = e.pool.QueryRow(context.Background(), `SELECT points_balance FROM customers WHERE id=$1`, cid).Scan(&bal)
	_ = e.pool.QueryRow(context.Background(), `SELECT stock FROM rewards WHERE id=$1`, rid).Scan(&stock)
	if ok != 1 || insuf != n-1 || bal != 10 || stock != 99 {
		t.Fatalf("ok=%d insuf=%d bal=%d stock=%d", ok, insuf, bal, stock)
	}
}
