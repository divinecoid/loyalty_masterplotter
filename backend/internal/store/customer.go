package store

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var (
	ErrClaimed     = errors.New("code already claimed")
	ErrNeedProfile = errors.New("name and phone required")
)

type Customer struct {
	ID            int64     `json:"-"`
	Name          string    `json:"name"`
	Phone         string    `json:"-"`
	Points        int       `json:"points"`
	PhoneVerified bool      `json:"phone_verified"`
	CreatedAt     time.Time `json:"created_at"`
}

// CustomerByDevice mengenali pembeli dari token perangkat (hash sha256).
func (s *Store) CustomerByDevice(ctx context.Context, tokenHash []byte) (Customer, error) {
	var c Customer
	err := s.pool.QueryRow(ctx,
		`UPDATE devices d SET last_seen=now() FROM customers c
		 WHERE d.token_hash=$1 AND c.id=d.customer_id
		 RETURNING c.id,c.name,c.phone_e164,c.points_balance,c.phone_verified,c.created_at`, tokenHash).
		Scan(&c.ID, &c.Name, &c.Phone, &c.Points, &c.PhoneVerified, &c.CreatedAt)
	return c, mapErr(err)
}

// ScanInfo: info publik sebuah kode. Status disederhanakan agar tidak membocorkan state internal.
type ScanInfo struct {
	State       string // ok | claimed | invalid
	ProductName string
	Points      int
	Mine        bool // sudah diklaim oleh perangkat ini
}

func (s *Store) ScanInfo(ctx context.Context, code uuid.UUID, deviceCustomerID int64) ScanInfo {
	var status, name string
	var points int
	var claimedBy *int64
	err := s.pool.QueryRow(ctx,
		`SELECT q.status,p.name,q.points,q.claimed_by FROM qr_codes q JOIN products p ON p.id=q.product_id
		 WHERE q.code=$1`, code).Scan(&status, &name, &points, &claimedBy)
	switch {
	case err != nil:
		return ScanInfo{State: "invalid"}
	case status == "active":
		return ScanInfo{State: "ok", ProductName: name, Points: points}
	case status == "claimed":
		return ScanInfo{State: "claimed", Mine: claimedBy != nil && deviceCustomerID != 0 && *claimedBy == deviceCustomerID}
	default: // generated / void => tidak bisa dibedakan dari kode asal-asalan
		return ScanInfo{State: "invalid"}
	}
}

type ClaimInput struct {
	Code          uuid.UUID
	DeviceHash    []byte // dari cookie (boleh nil)
	NewDeviceHash []byte // dipakai jika perangkat belum dikenali
	Name, Phone   string // phone sudah E.164; wajib jika perangkat belum dikenali
	Consent       bool
	IP, UA        string
}

type ClaimResult struct {
	Earned      int
	Balance     int
	Name        string
	NewDevice   bool // handler harus set cookie token baru
	NewCustomer bool
	Product     string
}

// Claim: klaim atomik. Baris kode dikunci (FOR UPDATE) sehingga hanya satu pemanggil yang bisa menang.
func (s *Store) Claim(ctx context.Context, in ClaimInput) (ClaimResult, error) {
	var res ClaimResult
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return res, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	var status string
	if err := tx.QueryRow(ctx,
		`SELECT q.status,q.points,p.name FROM qr_codes q JOIN products p ON p.id=q.product_id
		 WHERE q.code=$1 FOR UPDATE OF q`, in.Code).Scan(&status, &res.Earned, &res.Product); err != nil {
		return res, mapErr(err)
	}
	switch status {
	case "active":
	case "claimed":
		return res, ErrClaimed
	default:
		return res, ErrNotFound
	}

	var custID int64
	known := false
	if in.DeviceHash != nil {
		err := tx.QueryRow(ctx,
			`SELECT c.id,c.name FROM devices d JOIN customers c ON c.id=d.customer_id WHERE d.token_hash=$1`,
			in.DeviceHash).Scan(&custID, &res.Name)
		if err == nil {
			known = true
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return res, err
		}
	}
	if !known {
		if in.Name == "" || in.Phone == "" || !in.Consent {
			return res, ErrNeedProfile
		}
		// Upsert by nomor. Nama pelanggan lama tidak ditimpa.
		var created bool
		if err := tx.QueryRow(ctx,
			`INSERT INTO customers(name,phone_e164,consent_at) VALUES($1,$2,now())
			 ON CONFLICT (phone_e164) DO UPDATE SET phone_e164=EXCLUDED.phone_e164
			 RETURNING id,name,(xmax=0)`, in.Name, in.Phone).Scan(&custID, &res.Name, &created); err != nil {
			return res, err
		}
		res.NewCustomer = created
		if _, err := tx.Exec(ctx, `INSERT INTO devices(customer_id,token_hash) VALUES($1,$2)`,
			custID, in.NewDeviceHash); err != nil {
			return res, err
		}
		res.NewDevice = true
	}

	if _, err := tx.Exec(ctx,
		`UPDATE qr_codes SET status='claimed',claimed_by=$2,claimed_at=now(),claim_ip=NULLIF($3,'')::inet,claim_ua=$4
		 WHERE code=$1`, in.Code, custID, in.IP, truncateStr(in.UA, 300)); err != nil {
		return res, err
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO point_ledger(customer_id,delta,reason,qr_code) VALUES($1,$2,'scan',$3)`,
		custID, res.Earned, in.Code); err != nil {
		return res, err
	}
	if err := tx.QueryRow(ctx,
		`UPDATE customers SET points_balance=points_balance+$2 WHERE id=$1 RETURNING points_balance`,
		custID, res.Earned).Scan(&res.Balance); err != nil {
		return res, err
	}
	return res, tx.Commit(ctx)
}

type LedgerEntry struct {
	Delta     int       `json:"delta"`
	Reason    string    `json:"reason"`
	Product   *string   `json:"product"`
	CreatedAt time.Time `json:"created_at"`
}

func (s *Store) Ledger(ctx context.Context, customerID int64, limit int) ([]LedgerEntry, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT l.delta,l.reason,p.name,l.created_at FROM point_ledger l
		 LEFT JOIN qr_codes q ON q.code=l.qr_code LEFT JOIN products p ON p.id=q.product_id
		 WHERE l.customer_id=$1 ORDER BY l.id DESC LIMIT $2`, customerID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []LedgerEntry{}
	for rows.Next() {
		var e LedgerEntry
		if err := rows.Scan(&e.Delta, &e.Reason, &e.Product, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func truncateStr(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}
