package store

import (
	"context"
	"crypto/rand"
	"errors"
	"time"
)

var (
	ErrInsufficient = errors.New("insufficient points")
	ErrOutOfStock   = errors.New("out of stock")
	ErrNotVerified  = errors.New("phone not verified")
)

type Reward struct {
	ID         int64   `json:"id"`
	Name       string  `json:"name"`
	CostPoints int     `json:"cost_points"`
	Stock      int     `json:"stock"`
	ImageURL   *string `json:"image_url"`
}

// ListActiveRewards: katalog untuk pembeli (hanya reward aktif).
func (s *Store) ListActiveRewards(ctx context.Context) ([]Reward, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id,name,cost_points,stock,image_url FROM rewards WHERE active ORDER BY cost_points,id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Reward{}
	for rows.Next() {
		var r Reward
		if err := rows.Scan(&r.ID, &r.Name, &r.CostPoints, &r.Stock, &r.ImageURL); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

type Redemption struct {
	ID        int64     `json:"id"`
	Reward    string    `json:"reward"`
	Cost      int       `json:"cost"`
	Status    string    `json:"status"`
	Code      string    `json:"code"`
	CreatedAt time.Time `json:"created_at"`
}

type RedeemResult struct {
	Redemption Redemption
	Balance    int
}

const codeAlphabet = "23456789ABCDEFGHJKLMNPQRSTUVWXYZ" // tanpa 0/O/1/I

// newRedeemCode: "XXXX-XXXX" dari CSPRNG (32^8 kemungkinan).
func newRedeemCode() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	out := make([]byte, 0, 9)
	for i, x := range b {
		if i == 4 {
			out = append(out, '-')
		}
		out = append(out, codeAlphabet[int(x)%len(codeAlphabet)]) // 256 % 32 == 0 -> tanpa bias
	}
	return string(out), nil
}

// Redeem menukar poin dengan reward secara atomik. Urutan kunci selalu customer -> reward (tanpa deadlock).
// requireVerified: jika true, hanya pelanggan dengan phone_verified yang boleh menukar.
func (s *Store) Redeem(ctx context.Context, customerID, rewardID int64, requireVerified bool) (RedeemResult, error) {
	var res RedeemResult
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return res, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	var balance int
	var verified bool
	if err := tx.QueryRow(ctx,
		`SELECT points_balance,phone_verified FROM customers WHERE id=$1 FOR UPDATE`, customerID).
		Scan(&balance, &verified); err != nil {
		return res, mapErr(err)
	}
	if requireVerified && !verified {
		return res, ErrNotVerified
	}

	var name string
	var cost, stock int
	if err := tx.QueryRow(ctx,
		`SELECT name,cost_points,stock FROM rewards WHERE id=$1 AND active FOR UPDATE`, rewardID).
		Scan(&name, &cost, &stock); err != nil {
		return res, mapErr(err)
	}
	if stock <= 0 {
		return res, ErrOutOfStock
	}
	if balance < cost {
		return res, ErrInsufficient
	}

	var rid int64
	var code string
	var created time.Time
	for attempt := 0; ; attempt++ {
		if attempt >= 5 {
			return res, errors.New("gagal membuat kode penukaran")
		}
		if code, err = newRedeemCode(); err != nil {
			return res, err
		}
		err = tx.QueryRow(ctx,
			`INSERT INTO redemptions(customer_id,reward_id,cost,code) VALUES($1,$2,$3,$4)
			 ON CONFLICT (code) DO NOTHING RETURNING id,created_at`, customerID, rewardID, cost, code).
			Scan(&rid, &created)
		if err == nil {
			break
		}
		if !isNoRows(err) {
			return res, err
		}
	}
	if _, err := tx.Exec(ctx, `UPDATE rewards SET stock=stock-1 WHERE id=$1`, rewardID); err != nil {
		return res, err
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO point_ledger(customer_id,delta,reason,ref_id) VALUES($1,$2,'redeem',$3)`,
		customerID, -cost, rid); err != nil {
		return res, err
	}
	if err := tx.QueryRow(ctx,
		`UPDATE customers SET points_balance=points_balance-$2 WHERE id=$1 RETURNING points_balance`,
		customerID, cost).Scan(&res.Balance); err != nil {
		return res, err
	}
	res.Redemption = Redemption{ID: rid, Reward: name, Cost: cost, Status: "pending", Code: code, CreatedAt: created}
	return res, tx.Commit(ctx)
}

func (s *Store) CustomerRedemptions(ctx context.Context, customerID int64) ([]Redemption, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT r.id,w.name,r.cost,r.status,r.code,r.created_at FROM redemptions r
		 JOIN rewards w ON w.id=r.reward_id WHERE r.customer_id=$1 ORDER BY r.id DESC LIMIT 50`, customerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Redemption{}
	for rows.Next() {
		var x Redemption
		if err := rows.Scan(&x.ID, &x.Reward, &x.Cost, &x.Status, &x.Code, &x.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}
