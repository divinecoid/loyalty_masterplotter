package store

import (
	"context"
	"errors"
	"strings"
	"time"
)

var ErrBadDelta = errors.New("saldo tidak boleh negatif")

// ---------- Reward (admin) ----------

type AdminReward struct {
	ID         int64   `json:"id"`
	Name       string  `json:"name"`
	CostPoints int     `json:"cost_points"`
	Stock      int     `json:"stock"`
	ImageURL   *string `json:"image_url"`
	Active     bool    `json:"active"`
}

func (s *Store) ListAllRewards(ctx context.Context) ([]AdminReward, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id,name,cost_points,stock,image_url,active FROM rewards ORDER BY active DESC,cost_points,id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []AdminReward{}
	for rows.Next() {
		var r AdminReward
		if err := rows.Scan(&r.ID, &r.Name, &r.CostPoints, &r.Stock, &r.ImageURL, &r.Active); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) CreateReward(ctx context.Context, name string, cost, stock int, image *string) (AdminReward, error) {
	r := AdminReward{Name: name, CostPoints: cost, Stock: stock, ImageURL: image, Active: true}
	err := s.pool.QueryRow(ctx,
		`INSERT INTO rewards(name,cost_points,stock,image_url) VALUES($1,$2,$3,$4) RETURNING id`,
		name, cost, stock, image).Scan(&r.ID)
	return r, err
}

func (s *Store) UpdateReward(ctx context.Context, id int64, name *string, cost, stock *int, active *bool) (AdminReward, error) {
	var r AdminReward
	err := s.pool.QueryRow(ctx,
		`UPDATE rewards SET name=COALESCE($2,name), cost_points=COALESCE($3,cost_points),
		   stock=COALESCE($4,stock), active=COALESCE($5,active) WHERE id=$1
		 RETURNING id,name,cost_points,stock,image_url,active`, id, name, cost, stock, active).
		Scan(&r.ID, &r.Name, &r.CostPoints, &r.Stock, &r.ImageURL, &r.Active)
	return r, mapErr(err)
}

// ---------- Penukaran (admin) ----------

type AdminRedemption struct {
	ID          int64      `json:"id"`
	Code        string     `json:"code"`
	Status      string     `json:"status"`
	Reward      string     `json:"reward"`
	Cost        int        `json:"cost"`
	Customer    string     `json:"customer"`
	Phone       string     `json:"phone"`
	CreatedAt   time.Time  `json:"created_at"`
	FulfilledAt *time.Time `json:"fulfilled_at"`
}

// ListRedemptions: filter status (opsional) dan q (cocok ke kode / nama / nomor).
func (s *Store) ListRedemptions(ctx context.Context, status, q string, limit, offset int) ([]AdminRedemption, error) {
	q = strings.TrimSpace(q)
	rows, err := s.pool.Query(ctx,
		`SELECT r.id,r.code,r.status,w.name,r.cost,c.name,c.phone_e164,r.created_at,r.fulfilled_at
		 FROM redemptions r JOIN rewards w ON w.id=r.reward_id JOIN customers c ON c.id=r.customer_id
		 WHERE ($1::text='' OR r.status=$1)
		   AND ($2::text='' OR r.code ILIKE '%'||$2||'%' OR c.name ILIKE '%'||$2||'%' OR c.phone_e164 LIKE '%'||$2||'%')
		 ORDER BY r.id DESC LIMIT $3 OFFSET $4`, status, q, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []AdminRedemption{}
	for rows.Next() {
		var x AdminRedemption
		if err := rows.Scan(&x.ID, &x.Code, &x.Status, &x.Reward, &x.Cost, &x.Customer, &x.Phone, &x.CreatedAt, &x.FulfilledAt); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

// FulfillRedemption: pending -> fulfilled (idempoten terhadap race: hanya satu yang menang).
func (s *Store) FulfillRedemption(ctx context.Context, id int64) error {
	return affected(func() (int64, error) {
		ct, err := s.pool.Exec(ctx,
			`UPDATE redemptions SET status='fulfilled',fulfilled_at=now() WHERE id=$1 AND status='pending'`, id)
		return ct.RowsAffected(), err
	}())
}

// CancelRedemption: pending -> cancelled, poin dikembalikan lewat ledger ('adjust') dan stok +1.
// Urutan kunci: customer -> redemption -> reward (konsisten dengan Redeem).
func (s *Store) CancelRedemption(ctx context.Context, id int64) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	var cid, rewardID int64
	if err := tx.QueryRow(ctx, `SELECT customer_id,reward_id FROM redemptions WHERE id=$1`, id).Scan(&cid, &rewardID); err != nil {
		return mapErr(err)
	}
	if _, err := tx.Exec(ctx, `SELECT 1 FROM customers WHERE id=$1 FOR UPDATE`, cid); err != nil {
		return err
	}
	var status string
	var cost int
	if err := tx.QueryRow(ctx, `SELECT status,cost FROM redemptions WHERE id=$1 FOR UPDATE`, id).Scan(&status, &cost); err != nil {
		return mapErr(err)
	}
	if status != "pending" {
		return ErrConflict
	}
	if _, err := tx.Exec(ctx, `UPDATE redemptions SET status='cancelled' WHERE id=$1`, id); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE rewards SET stock=stock+1 WHERE id=$1`, rewardID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO point_ledger(customer_id,delta,reason,ref_id) VALUES($1,$2,'adjust',$3)`, cid, cost, id); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE customers SET points_balance=points_balance+$2 WHERE id=$1`, cid, cost); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// ---------- Pelanggan (admin) ----------

type AdminCustomer struct {
	ID            int64     `json:"id"`
	Name          string    `json:"name"`
	Phone         string    `json:"phone"`
	Points        int       `json:"points"`
	PhoneVerified bool      `json:"phone_verified"`
	CreatedAt     time.Time `json:"created_at"`
}

func (s *Store) ListCustomers(ctx context.Context, q string, limit, offset int) ([]AdminCustomer, error) {
	q = strings.TrimSpace(q)
	rows, err := s.pool.Query(ctx,
		`SELECT id,name,phone_e164,points_balance,phone_verified,created_at FROM customers
		 WHERE $1::text='' OR name ILIKE '%'||$1||'%' OR phone_e164 LIKE '%'||$1||'%'
		 ORDER BY id DESC LIMIT $2 OFFSET $3`, q, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []AdminCustomer{}
	for rows.Next() {
		var c AdminCustomer
		if err := rows.Scan(&c.ID, &c.Name, &c.Phone, &c.Points, &c.PhoneVerified, &c.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) SetPhoneVerified(ctx context.Context, id int64, v bool) error {
	ct, err := s.pool.Exec(ctx, `UPDATE customers SET phone_verified=$2 WHERE id=$1`, id, v)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// AdjustPoints: koreksi manual (delta +/-). Saldo tidak boleh jadi negatif. Mengembalikan saldo baru.
func (s *Store) AdjustPoints(ctx context.Context, id int64, delta int) (int, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	var bal int
	if err := tx.QueryRow(ctx, `SELECT points_balance FROM customers WHERE id=$1 FOR UPDATE`, id).Scan(&bal); err != nil {
		return 0, mapErr(err)
	}
	if bal+delta < 0 {
		return 0, ErrBadDelta
	}
	if _, err := tx.Exec(ctx, `INSERT INTO point_ledger(customer_id,delta,reason) VALUES($1,$2,'adjust')`, id, delta); err != nil {
		return 0, err
	}
	if err := tx.QueryRow(ctx, `UPDATE customers SET points_balance=points_balance+$2 WHERE id=$1 RETURNING points_balance`, id, delta).Scan(&bal); err != nil {
		return 0, err
	}
	return bal, tx.Commit(ctx)
}

// ---------- Ringkasan & audit ----------

type Summary struct {
	Customers          int `json:"customers"`
	VerifiedCustomers  int `json:"verified_customers"`
	CodesActive        int `json:"codes_active"`
	CodesClaimed       int `json:"codes_claimed"`
	PointsIssued       int `json:"points_issued"`
	PointsRedeemed     int `json:"points_redeemed"`
	RedemptionsPending int `json:"redemptions_pending"`
	RedemptionsDone    int `json:"redemptions_fulfilled"`
}

func (s *Store) Summary(ctx context.Context) (Summary, error) {
	var m Summary
	err := s.pool.QueryRow(ctx, `SELECT
	  (SELECT count(*) FROM customers),
	  (SELECT count(*) FROM customers WHERE phone_verified),
	  (SELECT count(*) FROM qr_codes WHERE status='active'),
	  (SELECT count(*) FROM qr_codes WHERE status='claimed'),
	  (SELECT COALESCE(sum(delta),0) FROM point_ledger WHERE reason='scan'),
	  (SELECT COALESCE(-sum(delta),0) FROM point_ledger WHERE reason='redeem'),
	  (SELECT count(*) FROM redemptions WHERE status='pending'),
	  (SELECT count(*) FROM redemptions WHERE status='fulfilled')`).
		Scan(&m.Customers, &m.VerifiedCustomers, &m.CodesActive, &m.CodesClaimed,
			&m.PointsIssued, &m.PointsRedeemed, &m.RedemptionsPending, &m.RedemptionsDone)
	return m, err
}

type AuditEntry struct {
	ID        int64     `json:"id"`
	Admin     *string   `json:"admin"`
	Action    string    `json:"action"`
	Payload   string    `json:"payload"`
	CreatedAt time.Time `json:"created_at"`
}

func (s *Store) ListAudit(ctx context.Context, limit, offset int) ([]AuditEntry, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT l.id,a.email,l.action,l.payload::text,l.created_at FROM audit_logs l
		 LEFT JOIN admins a ON a.id=l.admin_id ORDER BY l.id DESC LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []AuditEntry{}
	for rows.Next() {
		var e AuditEntry
		if err := rows.Scan(&e.ID, &e.Admin, &e.Action, &e.Payload, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
