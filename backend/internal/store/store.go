// Package store: akses data (pgx) untuk admin, produk, batch, dan kode QR.
package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/masterplotter/loyalty/internal/qr"
)

var (
	ErrNotFound = errors.New("not found")
	ErrConflict = errors.New("conflict") // state tidak mengizinkan operasi
)

type Store struct{ pool *pgxpool.Pool }

func New(p *pgxpool.Pool) *Store { return &Store{pool: p} }

// ---------- Admin ----------

type Admin struct {
	ID           int64  `json:"id"`
	Email        string `json:"email"`
	Role         string `json:"role"`
	PasswordHash string `json:"-"`
}

func (s *Store) CountAdmins(ctx context.Context) (n int, err error) {
	err = s.pool.QueryRow(ctx, `SELECT count(*) FROM admins`).Scan(&n)
	return
}

func (s *Store) CreateAdmin(ctx context.Context, email, hash, role string) (Admin, error) {
	a := Admin{Email: email, Role: role}
	err := s.pool.QueryRow(ctx,
		`INSERT INTO admins(email,password_hash,role) VALUES($1,$2,$3) RETURNING id`, email, hash, role).Scan(&a.ID)
	return a, err
}

func (s *Store) AdminByEmail(ctx context.Context, email string) (Admin, error) {
	var a Admin
	err := s.pool.QueryRow(ctx, `SELECT id,email,role,password_hash FROM admins WHERE email=$1`, email).
		Scan(&a.ID, &a.Email, &a.Role, &a.PasswordHash)
	return a, mapErr(err)
}

func (s *Store) AdminByID(ctx context.Context, id int64) (Admin, error) {
	var a Admin
	err := s.pool.QueryRow(ctx, `SELECT id,email,role,password_hash FROM admins WHERE id=$1`, id).
		Scan(&a.ID, &a.Email, &a.Role, &a.PasswordHash)
	return a, mapErr(err)
}

// ---------- Produk ----------

type Product struct {
	ID            int64     `json:"id"`
	SKU           string    `json:"sku"`
	Name          string    `json:"name"`
	PointsPerUnit int       `json:"points_per_unit"`
	Active        bool      `json:"active"`
	CreatedAt     time.Time `json:"created_at"`
}

func (s *Store) ListProducts(ctx context.Context) ([]Product, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id,sku,name,points_per_unit,active,created_at FROM products ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Product{}
	for rows.Next() {
		var p Product
		if err := rows.Scan(&p.ID, &p.SKU, &p.Name, &p.PointsPerUnit, &p.Active, &p.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Store) CreateProduct(ctx context.Context, sku, name string, points int) (Product, error) {
	var p Product
	err := s.pool.QueryRow(ctx,
		`INSERT INTO products(sku,name,points_per_unit) VALUES($1,$2,$3)
		 RETURNING id,sku,name,points_per_unit,active,created_at`, sku, name, points).
		Scan(&p.ID, &p.SKU, &p.Name, &p.PointsPerUnit, &p.Active, &p.CreatedAt)
	return p, err
}

// UpdateProduct: field nil = tidak diubah. Poin baru tidak mempengaruhi batch yang sudah dibuat (snapshot).
func (s *Store) UpdateProduct(ctx context.Context, id int64, name *string, points *int, active *bool) (Product, error) {
	var p Product
	err := s.pool.QueryRow(ctx,
		`UPDATE products SET name=COALESCE($2,name), points_per_unit=COALESCE($3,points_per_unit),
		        active=COALESCE($4,active) WHERE id=$1
		 RETURNING id,sku,name,points_per_unit,active,created_at`, id, name, points, active).
		Scan(&p.ID, &p.SKU, &p.Name, &p.PointsPerUnit, &p.Active, &p.CreatedAt)
	return p, mapErr(err)
}

// ---------- Batch ----------

type Batch struct {
	ID          int64          `json:"id"`
	ProductID   int64          `json:"product_id"`
	ProductName string         `json:"product_name"`
	Label       string         `json:"label"`
	Qty         int            `json:"qty"`
	Status      string         `json:"status"`
	CreatedAt   time.Time      `json:"created_at"`
	PrintedAt   *time.Time     `json:"printed_at"`
	Counts      map[string]int `json:"counts"`
}

const batchSelect = `
SELECT b.id,b.product_id,p.name,b.label,b.qty,b.status,b.created_at,b.printed_at,
  COALESCE(jsonb_object_agg(c.status,c.n) FILTER (WHERE c.status IS NOT NULL),'{}'::jsonb)
FROM batches b
JOIN products p ON p.id=b.product_id
LEFT JOIN (SELECT batch_id,status,count(*) n FROM qr_codes GROUP BY batch_id,status) c ON c.batch_id=b.id`

func scanBatch(row pgx.Row) (Batch, error) {
	var b Batch
	var counts []byte
	if err := row.Scan(&b.ID, &b.ProductID, &b.ProductName, &b.Label, &b.Qty, &b.Status,
		&b.CreatedAt, &b.PrintedAt, &counts); err != nil {
		return b, err
	}
	b.Counts = map[string]int{}
	return b, json.Unmarshal(counts, &b.Counts)
}

func (s *Store) ListBatches(ctx context.Context) ([]Batch, error) {
	rows, err := s.pool.Query(ctx, batchSelect+` GROUP BY b.id,p.name ORDER BY b.id DESC LIMIT 200`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Batch{}
	for rows.Next() {
		b, err := scanBatch(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

func (s *Store) BatchByID(ctx context.Context, id int64) (Batch, error) {
	b, err := scanBatch(s.pool.QueryRow(ctx, batchSelect+` WHERE b.id=$1 GROUP BY b.id,p.name`, id))
	return b, mapErr(err)
}

// CreateBatch membuat batch + qty kode UUID v4 (poin di-snapshot dari produk) dalam satu transaksi.
func (s *Store) CreateBatch(ctx context.Context, productID int64, label string, qty int, adminID int64) (int64, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	var points int
	var active bool
	if err := tx.QueryRow(ctx, `SELECT points_per_unit,active FROM products WHERE id=$1`, productID).
		Scan(&points, &active); err != nil {
		return 0, mapErr(err)
	}
	if !active {
		return 0, ErrConflict
	}
	var batchID int64
	if err := tx.QueryRow(ctx,
		`INSERT INTO batches(product_id,label,qty,created_by) VALUES($1,$2,$3,$4) RETURNING id`,
		productID, label, qty, adminID).Scan(&batchID); err != nil {
		return 0, err
	}
	rows := make([][]any, qty)
	for i := range rows {
		rows[i] = []any{qr.NewCode(), batchID, i + 1, productID, points}
	}
	if _, err := tx.CopyFrom(ctx, pgx.Identifier{"qr_codes"},
		[]string{"code", "batch_id", "seq", "product_id", "points"}, pgx.CopyFromRows(rows)); err != nil {
		return 0, err
	}
	return batchID, tx.Commit(ctx)
}

// BatchCodes mengembalikan kode batch berurutan seq (nomor urut stabil di stiker).
func (s *Store) BatchCodes(ctx context.Context, batchID int64) ([]uuid.UUID, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT code FROM qr_codes WHERE batch_id=$1 ORDER BY seq`, batchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []uuid.UUID
	for rows.Next() {
		var u uuid.UUID
		if err := rows.Scan(&u); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// MarkPrinted: generated -> printed.
func (s *Store) MarkPrinted(ctx context.Context, batchID int64) error {
	tag, err := s.pool.Exec(ctx,
		`UPDATE batches SET status='printed', printed_at=now() WHERE id=$1 AND status='generated'`, batchID)
	return affected(tag.RowsAffected(), err)
}

// ActivateBatch: printed -> active; kode generated -> active (baru sekarang bisa diklaim pembeli).
func (s *Store) ActivateBatch(ctx context.Context, batchID int64) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	tag, err := tx.Exec(ctx, `UPDATE batches SET status='active' WHERE id=$1 AND status='printed'`, batchID)
	if err := affected(tag.RowsAffected(), err); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx,
		`UPDATE qr_codes SET status='active' WHERE batch_id=$1 AND status='generated'`, batchID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// ---------- Kode ----------

type Code struct {
	Code      uuid.UUID  `json:"code"`
	BatchID   int64      `json:"batch_id"`
	Product   string     `json:"product"`
	Points    int        `json:"points"`
	Status    string     `json:"status"`
	ClaimedAt *time.Time `json:"claimed_at"`
}

func (s *Store) ListCodes(ctx context.Context, status string, batchID int64, limit, offset int) ([]Code, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT q.code,q.batch_id,p.name,q.points,q.status,q.claimed_at
		 FROM qr_codes q JOIN products p ON p.id=q.product_id
		 WHERE ($1::text='' OR q.status=$1::text) AND ($2::bigint=0 OR q.batch_id=$2::bigint)
		 ORDER BY q.claimed_at DESC NULLS LAST, q.batch_id DESC LIMIT $3 OFFSET $4`,
		status, batchID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Code{}
	for rows.Next() {
		var c Code
		if err := rows.Scan(&c.Code, &c.BatchID, &c.Product, &c.Points, &c.Status, &c.ClaimedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// VoidCode: hanya kode yang belum diklaim yang bisa dibatalkan.
func (s *Store) VoidCode(ctx context.Context, code uuid.UUID) error {
	tag, err := s.pool.Exec(ctx,
		`UPDATE qr_codes SET status='void' WHERE code=$1 AND status IN ('generated','active')`, code)
	return affected(tag.RowsAffected(), err)
}

// ---------- Audit ----------

func (s *Store) Audit(ctx context.Context, adminID int64, action string, payload any) {
	b, _ := json.Marshal(payload)
	_, _ = s.pool.Exec(ctx, `INSERT INTO audit_logs(admin_id,action,payload) VALUES($1,$2,$3)`, adminID, action, b)
}

// ---------- util ----------

func isNoRows(err error) bool { return errors.Is(err, pgx.ErrNoRows) }

func mapErr(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

// affected: 0 baris => state tidak cocok (conflict) atau tidak ada.
func affected(n int64, err error) error {
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrConflict
	}
	return nil
}

// UpsertAdmin membuat admin atau mengganti password_hash (dan role) bila email sudah ada.
func (s *Store) UpsertAdmin(ctx context.Context, email, hash, role string) (created bool, err error) {
	err = s.pool.QueryRow(ctx,
		`INSERT INTO admins(email,password_hash,role) VALUES($1,$2,$3)
		 ON CONFLICT (email) DO UPDATE SET password_hash=EXCLUDED.password_hash, role=EXCLUDED.role
		 RETURNING (xmax = 0)`, email, hash, role).Scan(&created)
	return
}
