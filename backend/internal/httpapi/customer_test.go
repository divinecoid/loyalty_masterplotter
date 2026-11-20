package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/masterplotter/loyalty/internal/config"
	"github.com/masterplotter/loyalty/internal/db"
	"github.com/masterplotter/loyalty/internal/store"
)

type env struct {
	pool  *pgxpool.Pool
	st    *store.Store
	codes []uuid.UUID // batch: 6 kode @10 poin, status generated
	batch int64
}

func setup(t *testing.T) env {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL tidak di-set")
	}
	ctx := context.Background()
	pool, err := db.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := db.Migrate(pool); err != nil {
		t.Fatal(err)
	}
	st := store.New(pool)
	adm, err := st.CreateAdmin(ctx, "c-"+uuid.NewString()[:8]+"@test.local", "x", "owner")
	if err != nil {
		t.Fatal(err)
	}
	p, err := st.CreateProduct(ctx, "SKU-"+uuid.NewString()[:8], "Kertas Sublimasi", 10)
	if err != nil {
		t.Fatal(err)
	}
	bid, err := st.CreateBatch(ctx, p.ID, "uji", 6, adm.ID)
	if err != nil {
		t.Fatal(err)
	}
	codes, err := st.BatchCodes(ctx, bid)
	if err != nil {
		t.Fatal(err)
	}
	return env{pool, st, codes, bid}
}

func (e env) activate(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	if err := e.st.MarkPrinted(ctx, e.batch); err != nil {
		t.Fatal(err)
	}
	if err := e.st.ActivateBatch(ctx, e.batch); err != nil {
		t.Fatal(err)
	}
}

// Satu "perangkat" = satu cookie jar sederhana.
type client struct {
	t   *testing.T
	app interface {
		Test(*http.Request, ...int) (*http.Response, error)
	}
	cookie *http.Cookie
}

func (c *client) do(method, path string, body any, want int) map[string]any {
	c.t.Helper()
	var rd io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, path, rd)
	req.Header.Set("Content-Type", "application/json")
	if c.cookie != nil {
		req.AddCookie(c.cookie)
	}
	resp, err := c.app.Test(req, 20000)
	if err != nil {
		c.t.Fatal(err)
	}
	out, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != want {
		c.t.Fatalf("%s %s: want %d got %d: %s", method, path, want, resp.StatusCode, out)
	}
	for _, ck := range resp.Cookies() {
		if ck.Name == deviceCookie || ck.Name == cookieName {
			c.cookie = ck
		}
	}
	m := map[string]any{}
	_ = json.Unmarshal(out, &m)
	return m
}

func TestCustomerFlow(t *testing.T) {
	e := setup(t)
	cfg := config.Config{JWTSecret: "test-secret-test-secret-test-secret", PublicURL: "https://loyalty.test", CORSOrigins: "http://localhost:5173"}
	app := New(cfg, e.st, func() error { return e.pool.Ping(context.Background()) })
	phoneA := fmt.Sprintf("0812%08d", uuid.New().ID()%100000000)
	scan := func(i int) string { return "/api/v1/scan/" + e.codes[i].String() }

	d1 := &client{t: t, app: app}
	if m := d1.do("GET", "/api/v1/scan/"+uuid.NewString(), nil, 200); m["state"] != "invalid" {
		t.Fatalf("kode acak: %v", m)
	}
	d1.do("GET", "/api/v1/scan/bukan-uuid", nil, 200)
	if m := d1.do("GET", scan(0), nil, 200); m["state"] != "invalid" {
		t.Fatalf("kode belum aktif harus tampak invalid: %v", m)
	}
	d1.do("POST", scan(0)+"/claim", map[string]any{"name": "Budi", "phone": phoneA, "consent": true}, 404) // belum aktif

	e.activate(t)
	if m := d1.do("GET", scan(0), nil, 200); m["state"] != "ok" || m["points"].(float64) != 10 || m["known"] != false {
		t.Fatalf("scan aktif: %v", m)
	}

	d1.do("POST", scan(0)+"/claim", nil, 400)                                                               // tanpa profil
	d1.do("POST", scan(0)+"/claim", map[string]any{"name": "Budi", "phone": "123", "consent": true}, 400)   // nomor buruk
	d1.do("POST", scan(0)+"/claim", map[string]any{"name": "B", "phone": phoneA, "consent": true}, 400)     // nama terlalu pendek
	d1.do("POST", scan(0)+"/claim", map[string]any{"name": "Budi", "phone": phoneA, "consent": false}, 400) // tanpa consent
	d1.do("GET", "/api/v1/me", nil, 401)

	m := d1.do("POST", scan(0)+"/claim", map[string]any{"name": "Budi", "phone": phoneA, "consent": true}, 200)
	if m["earned"].(float64) != 10 || m["balance"].(float64) != 10 || m["new_customer"] != true || d1.cookie == nil {
		t.Fatalf("klaim pertama: %v cookie=%v", m, d1.cookie)
	}
	if !d1.cookie.HttpOnly {
		t.Fatal("cookie perangkat harus HttpOnly")
	}
	me := d1.do("GET", "/api/v1/me", nil, 200)
	if me["points"].(float64) != 10 || me["name"] != "Budi" || me["phone_verified"] != false {
		t.Fatalf("me: %v", me)
	}
	if p := me["phone"].(string); p == "" || bytes.Contains([]byte(p), []byte(phoneA[4:8])) {
		t.Fatalf("nomor harus disamarkan: %s", p)
	}

	// kode yang sama: pemilik tahu "mine", orang lain tidak; klaim ulang ditolak
	if m := d1.do("GET", scan(0), nil, 200); m["state"] != "claimed" || m["mine"] != true {
		t.Fatalf("scan ulang pemilik: %v", m)
	}
	stranger := &client{t: t, app: app}
	if m := stranger.do("GET", scan(0), nil, 200); m["state"] != "claimed" || m["mine"] == true {
		t.Fatalf("scan orang lain: %v", m)
	}
	d1.do("POST", scan(0)+"/claim", nil, 409)
	stranger.do("POST", scan(0)+"/claim", map[string]any{"name": "Siti", "phone": "08111234567", "consent": true}, 409)

	// perangkat dikenali: klaim tanpa form
	if m := d1.do("POST", scan(1)+"/claim", nil, 200); m["balance"].(float64) != 20 || m["new_customer"] != false {
		t.Fatalf("klaim kedua: %v", m)
	}

	// perangkat baru, nomor sama (format beda) -> masuk ke akun yang sama, nama lama tidak ditimpa
	d2 := &client{t: t, app: app}
	m = d2.do("POST", scan(2)+"/claim", map[string]any{"name": "Nama Lain", "phone": "+62 " + phoneA[1:], "consent": true}, 200)
	if m["balance"].(float64) != 30 || m["new_customer"] != false || m["name"] != "Budi" {
		t.Fatalf("perangkat baru: %v", m)
	}

	ledger := d1.do("GET", "/api/v1/me/ledger", nil, 200)
	_ = ledger
	var n int
	if err := e.pool.QueryRow(context.Background(),
		`SELECT count(*) FROM point_ledger WHERE qr_code = ANY($1)`, e.codes).Scan(&n); err != nil || n != 3 {
		t.Fatalf("ledger rows=%d err=%v", n, err)
	}
}

// Balapan klaim: 20 pembeli berbeda memindai kode yang sama bersamaan -> tepat satu menang.
func TestClaimRaceExactlyOneWinner(t *testing.T) {
	e := setup(t)
	e.activate(t)
	ctx := context.Background()
	code := e.codes[0]

	const n = 20
	var wg sync.WaitGroup
	results := make([]error, n)
	phones := make([]string, n)
	for i := 0; i < n; i++ {
		phones[i] = fmt.Sprintf("+62899%08d", uuid.New().ID()%100000000)
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, hash, _ := newDeviceToken()
			_, results[i] = e.st.Claim(ctx, store.ClaimInput{
				Code: code, NewDeviceHash: hash, Name: fmt.Sprintf("User %d", i), Phone: phones[i], Consent: true,
			})
		}()
	}
	wg.Wait()

	wins, claimed := 0, 0
	for _, err := range results {
		switch err {
		case nil:
			wins++
		case store.ErrClaimed:
			claimed++
		default:
			t.Fatalf("error tak terduga: %v", err)
		}
	}
	if wins != 1 || claimed != n-1 {
		t.Fatalf("wins=%d claimed=%d", wins, claimed)
	}
	var ledger, custs int
	_ = e.pool.QueryRow(ctx, `SELECT count(*) FROM point_ledger WHERE qr_code=$1`, code).Scan(&ledger)
	_ = e.pool.QueryRow(ctx, `SELECT count(*) FROM customers WHERE phone_e164 = ANY($1)`, phones).Scan(&custs)
	if ledger != 1 || custs != 1 {
		t.Fatalf("ledger=%d customers dibuat=%d (yang kalah harus rollback)", ledger, custs)
	}
}
