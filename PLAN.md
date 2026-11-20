# Master Plotter Loyalty — Technical Plan

Status: draft v1 · Stack: Go Fiber + PostgreSQL · SvelteKit + shadcn-svelte

## 1. Ringkasan produk

Master Plotter (dealer printer/plotter Epson, konsumabel: tinta, kertas, sparepart) membuat program loyalty:

1. **Admin** generate stiker QR berhologram (UUID unik per unit) → dicetak & ditempel ke barang.
2. **Pembeli** scan QR dengan kamera HP → buka web app (tanpa install).
3. Scan **pertama kali**: isi nama + nomor WhatsApp → akun dibuat → poin masuk.
4. Scan berikutnya: otomatis dikenali → poin bertambah.
5. Poin ditukar reward (voucher, diskon, merchandise).

> Catatan istilah: "barcode" di sini = **QR code** (lebih mudah di-scan kamera HP bawaan, bisa memuat URL). Hologram adalah lapisan fisik stiker (dikerjakan vendor cetak), app menyediakan file siap cetak.

## 2. Brand / design tokens (diambil dari masterplotter.com)

| Token | Nilai | Dipakai untuk |
|---|---|---|
| `navy` | `#112F60` | Primary, header, heading, tombol utama |
| `navy-deep` | `#1B2D5E` | Hover/active primary, footer |
| `yellow` | `#FEC630` | Accent / CTA, banner, badge poin |
| `cyan` | `#05A8F8` | Link, info, highlight sekunder |
| `mist` | `#C9D1E6` | Teks sekunder di atas navy, border lembut |
| `ink` | `#333333` | Teks body |
| `line` | `#E0E0E0` | Border, divider |
| `wa-green` | `#25D366` | Tombol WhatsApp saja |
| Font | **Inter** | Seluruh UI (sama dengan heading situs) |
| Radius | `6px` (`--radius: 0.375rem`) | Tombol/card |

Mapping ke shadcn (`app.css`): `--primary` = navy, `--primary-foreground` = putih, `--accent` = yellow, `--accent-foreground` = navy (**bukan putih** — kontras putih di atas kuning kurang), `--ring` = cyan, `--border` = line, `--foreground` = ink. Header berwarna navy dengan strip kuning tipis di bawahnya, seperti situs aslinya. Logo: `https://www.masterplotter.com/wp-content/uploads/2026/09/logo.png` (minta file vektor ke owner untuk produksi).

## 3. Arsitektur

```
 Pembeli (HP) ──scan QR──► https://loyalty.<domain>/s/{uuid}
                                   │
                    SvelteKit (SPA/PWA, adapter-static)
                                   │  JSON/HTTPS (cookie httpOnly)
                           Go Fiber API  ──► PostgreSQL
                                   │
                              (opsional) Redis: rate limit, cache
                                   │
                   (fase 2) WhatsApp gateway: OTP & notifikasi
```

- **Backend**: Go 1.23+, Fiber v2 (v3 jika sudah stabil di tim), `pgx/v5` + `sqlc`, `goose` untuk migrasi, `go-playground/validator`, `zerolog`, `google/uuid`, `skip2/go-qrcode` (atau `yeqown/go-qrcode`) untuk QR, `golang-jwt` untuk admin.
- **Frontend**: SvelteKit + Svelte 5, Tailwind, **shadcn-svelte** (Bits UI), `sveltekit-superforms` + `zod` untuk form, `@zxing/browser` hanya jika ingin scanner dalam-app (default: kamera HP bawaan).
- **Satu repo monorepo**: `/backend`, `/web` (customer PWA + admin dalam satu app, dipisah route group `(customer)` dan `(admin)`).
- **Deploy**: Docker Compose (api, web/nginx, postgres), reverse proxy Caddy/Nginx + TLS. Backup Postgres harian.

## 4. Keamanan kode QR (inti permintaan)

- Kode = **UUID v4** (122 bit acak dari CSPRNG) → tidak bisa ditebak/di-increment. *Bukan* v1/v7 (mengandung waktu/urutan, bisa diprediksi sebagian).
- QR berisi URL: `https://loyalty.<domain>/s/{uuid}`. Tidak ada ID berurutan di mana pun yang terekspos (PK internal boleh `bigint`, tapi API publik hanya memakai UUID).
- **Klaim sekali pakai**: `UPDATE ... WHERE code=$1 AND status='active'` atomik di dalam transaksi, `claimed_by` + `claimed_at` terisi → scan kedua oleh orang lain ditolak ("sudah digunakan"). Mencegah race condition ganda-klaim.
- **Status siklus hidup**: `generated → printed → active → claimed` (+ `void`). Kode yang baru digenerate **belum bisa diklaim** sampai admin menandai batch sudah dicetak/ditempel (`active`). Ini mencegah kode bocor dari vendor cetak sebelum dipakai.
- **Rate limiting** endpoint scan/klaim per IP & per device (mis. 10 percobaan/menit); setelah N kali UUID tidak dikenal → blok sementara. Respon "tidak ditemukan" dibuat seragam (tidak membedakan salah/format).
- Foto/fotokopi QR: dilawan oleh hologram fisik (sulit ditiru) + aturan klaim sekali. Residual risk: orang yang membuka stiker lalu memotret sebelum pembeli — mitigasi: stiker tipe *void/tamper-evident* atau QR ditempel di bagian dalam kemasan.
- Admin: password argon2id, JWT pendek + refresh, role (owner, staff), audit log.
- Semua query parameterized (sqlc), CORS ketat, CSP, cookie `HttpOnly; Secure; SameSite=Lax`.

## 5. Model data (PostgreSQL)

```sql
products(id, sku, name, points_per_unit, active, created_at)

batches(id, product_id, qty, label, status,      -- generated|printed|active
        created_by, created_at, printed_at)

qr_codes(
  code uuid PRIMARY KEY DEFAULT gen_random_uuid(),   -- yang dicetak
  batch_id, product_id,
  points int NOT NULL,                               -- snapshot poin saat generate
  status text NOT NULL,                              -- generated|active|claimed|void
  claimed_by bigint NULL REFERENCES customers,
  claimed_at timestamptz NULL, claim_ip inet, claim_ua text
)

customers(id, name, phone_e164 UNIQUE, points_balance,
          phone_verified bool, created_at)

devices(id, customer_id, token_hash, last_seen)     -- sesi "dikenali" di HP

point_ledger(id, customer_id, delta int, reason,    -- scan|redeem|adjust|expire
             qr_code uuid NULL UNIQUE, ref_id, created_at)
  -- saldo = SUM(delta); customers.points_balance hanya cache, ledger = sumber kebenaran

rewards(id, name, cost_points, stock, image, active)
redemptions(id, customer_id, reward_id, cost, status, code, created_at, fulfilled_at)

admins(id, email, password_hash, role)
audit_logs(id, admin_id, action, payload jsonb, created_at)
```

Nomor WA dinormalisasi ke E.164 (`08xx` → `+628xx`) sebelum disimpan; `UNIQUE` pada `phone_e164`.

## 6. Alur utama

**A. Scan pertama (belum punya akun)**
1. `GET /s/{uuid}` (halaman web) → memanggil `GET /api/v1/scan/{uuid}` untuk cek status (valid / sudah diklaim / tidak ada). Tampilkan nama produk + poin yang akan didapat.
2. Pembeli isi **nama + nomor WhatsApp** → `POST /api/v1/scan/{uuid}/claim`.
3. Server (1 transaksi): upsert customer, buat device token (cookie), klaim QR atomik, tulis `point_ledger`.
4. Tampilkan "Selamat! +N poin" + saldo.

**B. Scan berikutnya**: cookie device dikenali → tombol "Klaim poin" langsung (tanpa form).

**C. Ganti HP / cookie hilang**: input nomor WA → fase 1: masuk dengan konfirmasi (risiko diketahui, lihat §10); fase 2: **OTP via WhatsApp**.

**D. Tukar reward**: pilih reward → potong poin (ledger negatif, cek saldo di transaksi) → kode redeem → admin verifikasi saat penyerahan.

**E. Admin**: CRUD produk → buat batch (qty N) → generate N UUID → **unduh PDF/SVG lembar stiker** (QR + teks pendek + logo, layout sesuai ukuran stiker hologram) → tandai *printed/active* → pantau scan, klaim, anomali.

## 7. API (ringkas, prefix `/api/v1`)

Publik/pembeli
- `GET  /scan/:uuid` — status kode (+ info produk/poin)
- `POST /scan/:uuid/claim` — `{name?, phone?}` (nama/phone wajib jika belum dikenali)
- `GET  /me` · `GET /me/ledger` · `GET /rewards` · `POST /rewards/:id/redeem`
- `POST /auth/recover` (fase 2: kirim & verifikasi OTP)

Admin (`/admin/*`, JWT)
- `POST /auth/login` · `GET/POST/PATCH /products`
- `POST /batches` (generate) · `GET /batches/:id/sheet.pdf` · `POST /batches/:id/activate`
- `GET /codes?status=` · `POST /codes/:uuid/void`
- `GET/POST/PATCH /rewards` · `GET /redemptions` · `PATCH /redemptions/:id/fulfill`
- `GET /customers` · `POST /customers/:id/adjust` · `GET /reports/summary` · `GET /audit-logs`

## 8. Struktur repo

```
backend/
  cmd/api/main.go
  internal/{config,db,http(handlers,middleware),service,qr,auth}/
  db/{migrations,queries}   # goose + sqlc
web/
  src/routes/
    (customer)/s/[uuid]/+page.svelte   # scan & claim
    (customer)/me, rewards
    (admin)/admin/{login,products,batches,codes,rewards,customers,reports}
  src/lib/{components/ui (shadcn), api.ts, stores}
docker-compose.yml  Makefile  README.md
```

## 9. Milestone

| # | Isi | Estimasi |
|---|---|---|
| M0 | Setup monorepo, Docker, migrasi, CI, theme shadcn (token §2) | 2 hari |
| M1 | Admin auth, produk, generate batch UUID, **export PDF stiker**, activate | 4–5 hari |
| M2 | Flow scan & klaim (first-time form, device cookie, ledger, anti race/abuse) | 4–5 hari |
| M3 | Halaman pembeli: saldo, riwayat, katalog reward, redeem | 3–4 hari |
| M4 | Admin: reward, redemption, customer, laporan, audit log | 3–4 hari |
| M5 | Hardening (rate limit, pen-test sederhana), PWA, load test scan, deploy | 3 hari |
| M6 (opsional) | OTP & notifikasi WhatsApp, tier member, poin kedaluwarsa | 4–6 hari |

Total MVP (M0–M5) ≈ 3–4 minggu satu developer.

## 10. Pertanyaan terbuka (butuh keputusan owner)

1. **Verifikasi nomor WA**: tanpa OTP (cepat, tapi nomor bisa dipalsukan/diklaim orang lain) atau OTP WhatsApp (aman, ada biaya gateway per pesan)? *Rekomendasi*: MVP tanpa OTP, OTP wajib sebelum redeem reward.
2. **Poin**: tetap per produk, per SKU, atau per nilai rupiah? Ada kedaluwarsa?
3. **Hologram**: vendor cetak mana & ukuran stiker (menentukan layout PDF)? Apakah QR dicetak di atas hologram atau di sampingnya?
4. **Reward**: bentuknya apa (voucher belanja, diskon servis, merchandise)? Siapa yang menyerahkan?
5. **Domain** (mis. `loyalty.masterplotter.com`) dan akses ke logo vektor/brand guideline resmi.
6. **Barang bernilai tinggi** (printer jutaan rupiah): ikut program poin atau pakai alur registrasi garansi terpisah?
7. Kepatuhan data pribadi (UU PDP): perlu consent checkbox + kebijakan privasi saat input nama & WA.
