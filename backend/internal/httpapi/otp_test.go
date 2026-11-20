package httpapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"sync"
	"testing"
	"time"

	"github.com/masterplotter/loyalty/internal/config"
	"github.com/masterplotter/loyalty/internal/store"
)

type capture struct {
	mu   sync.Mutex
	msgs []string
	to   []string
	fail bool
}

func (c *capture) Send(_ context.Context, to, text string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.fail {
		return errors.New("gateway down")
	}
	c.to = append(c.to, to)
	c.msgs = append(c.msgs, text)
	return nil
}
func (c *capture) count() int { c.mu.Lock(); defer c.mu.Unlock(); return len(c.msgs) }
func (c *capture) last() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.msgs) == 0 {
		return ""
	}
	return c.msgs[len(c.msgs)-1]
}

var sixDigits = regexp.MustCompile(`\b\d{6}\b`)

func TestOTPHTTP(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	cfg := config.Config{JWTSecret: "test-secret-test-secret-test-secret", CORSOrigins: "http://localhost:5173", RedeemRequiresVerified: true}
	cap := &capture{}
	app := New(cfg, e.st, func() error { return e.pool.Ping(ctx) }, WithSender(cap))
	cid, tok := e.newCustomer(t, 20, false)
	cu := &client{t: t, app: app, cookie: &http.Cookie{Name: deviceCookie, Value: tok}}

	(&client{t: t, app: app}).do("POST", "/api/v1/me/verify/request", nil, 401)
	if m := cu.do("GET", "/api/v1/me", nil, 200); m["otp_available"] != true {
		t.Fatalf("otp_available: %v", m)
	}

	cu.do("POST", "/api/v1/me/verify/request", nil, 200)
	code := sixDigits.FindString(cap.last())
	if code == "" {
		t.Fatalf("OTP tidak terkirim: %q", cap.last())
	}
	cu.do("POST", "/api/v1/me/verify/request", nil, 429) // cooldown
	wrong := "000000"
	if code == wrong {
		wrong = "000001"
	}
	cu.do("POST", "/api/v1/me/verify/confirm", map[string]any{"code": "12ab"}, 400)
	cu.do("POST", "/api/v1/me/verify/confirm", map[string]any{"code": wrong}, 400)

	var verified bool
	e.pool.QueryRow(ctx, `SELECT phone_verified FROM customers WHERE id=$1`, cid).Scan(&verified)
	if verified {
		t.Fatal("terverifikasi sebelum kode benar")
	}
	cu.do("POST", "/api/v1/me/verify/confirm", map[string]any{"code": code}, 200)
	e.pool.QueryRow(ctx, `SELECT phone_verified FROM customers WHERE id=$1`, cid).Scan(&verified)
	if !verified {
		t.Fatal("harusnya terverifikasi")
	}
	// kode sekali pakai & sudah terverifikasi
	cu.do("POST", "/api/v1/me/verify/confirm", map[string]any{"code": code}, 400)
	cu.do("POST", "/api/v1/me/verify/request", nil, 409)

	// notifikasi penukaran dikirim ke nomor terverifikasi
	rid := e.newReward(t, 10, 1)
	before := cap.count()
	cu.do("POST", fmt.Sprintf("/api/v1/rewards/%d/redeem", rid), nil, 201)
	deadline := time.Now().Add(3 * time.Second)
	for cap.count() == before && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if cap.count() == before {
		t.Fatal("notifikasi penukaran tidak terkirim")
	}
}

func TestOTPDisabledAndGatewayDown(t *testing.T) {
	e := setup(t)
	cfg := config.Config{JWTSecret: "test-secret-test-secret-test-secret", CORSOrigins: "http://localhost:5173", Production: true}
	app := New(cfg, e.st, func() error { return nil }) // produksi tanpa webhook => nil sender
	_, tok := e.newCustomer(t, 0, false)
	cu := &client{t: t, app: app, cookie: &http.Cookie{Name: deviceCookie, Value: tok}}
	if m := cu.do("GET", "/api/v1/me", nil, 200); m["otp_available"] != false {
		t.Fatalf("otp_available: %v", m)
	}
	cu.do("POST", "/api/v1/me/verify/request", nil, 503)

	app2 := New(cfg, e.st, func() error { return nil }, WithSender(&capture{fail: true}))
	cu2 := &client{t: t, app: app2, cookie: &http.Cookie{Name: deviceCookie, Value: tok}}
	cu2.do("POST", "/api/v1/me/verify/request", nil, 502)
}

func TestOTPStoreRules(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	h := []byte("hash-benar-0123456789")
	bad := []byte("hash-salah-0123456789")

	// cooldown
	c1, _ := e.newCustomer(t, 0, false)
	if err := e.st.CreateOTP(ctx, c1, h, time.Minute); err != nil {
		t.Fatal(err)
	}
	if err := e.st.CreateOTP(ctx, c1, h, time.Minute); !errors.Is(err, store.ErrOTPTooSoon) {
		t.Fatalf("cooldown: %v", err)
	}

	// brute force: 5 salah mengunci, kode benar pun ditolak setelahnya
	for i := 1; i <= 5; i++ {
		err := e.st.VerifyOTP(ctx, c1, bad)
		want := store.ErrOTPInvalid
		if i == 5 {
			want = store.ErrOTPLocked
		}
		if !errors.Is(err, want) {
			t.Fatalf("percobaan %d: %v want %v", i, err, want)
		}
	}
	if err := e.st.VerifyOTP(ctx, c1, h); !errors.Is(err, store.ErrOTPLocked) {
		t.Fatalf("setelah terkunci: %v", err)
	}
	var v bool
	e.pool.QueryRow(ctx, `SELECT phone_verified FROM customers WHERE id=$1`, c1).Scan(&v)
	if v {
		t.Fatal("tidak boleh terverifikasi")
	}

	// kedaluwarsa
	c2, _ := e.newCustomer(t, 0, false)
	if err := e.st.CreateOTP(ctx, c2, h, -time.Second); err != nil {
		t.Fatal(err)
	}
	if err := e.st.VerifyOTP(ctx, c2, h); !errors.Is(err, store.ErrOTPInvalid) {
		t.Fatalf("kedaluwarsa: %v", err)
	}

	// tanpa kode sama sekali
	c3, _ := e.newCustomer(t, 0, false)
	if err := e.st.VerifyOTP(ctx, c3, h); !errors.Is(err, store.ErrOTPInvalid) {
		t.Fatalf("tanpa kode: %v", err)
	}

	// balapan: 10 verifikasi benar bersamaan -> tidak ada error aneh, hasil akhir terverifikasi
	c4, _ := e.newCustomer(t, 0, false)
	if err := e.st.CreateOTP(ctx, c4, h, time.Minute); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	var mu sync.Mutex
	ok := 0
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if e.st.VerifyOTP(ctx, c4, h) == nil {
				mu.Lock()
				ok++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if ok != 1 {
		t.Fatalf("sukses=%d want 1 (kode sekali pakai)", ok)
	}
}
