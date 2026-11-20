# Master Plotter Loyalty

Lihat [PLAN.md](PLAN.md) untuk technical plan lengkap.

## Jalankan lokal

```powershell
docker compose up -d db                      # Postgres
cd backend
$env:ADMIN_EMAIL="owner@masterplotter.local"; $env:ADMIN_PASSWORD="<isi-password-kuat>"   # admin pertama (hanya jika tabel admins kosong)
go run ./cmd/api                             # migrasi jalan otomatis; API :8080 (cek: /api/v1/health)

cd ../web
npm install --engine-strict=false
npm run dev                                  # Web :5173, /api diproxy ke :8080
```

Admin UI: http://localhost:5173/admin (login → Produk → Batch & Stiker → unduh PDF → Tandai tercetak → Aktifkan).

## Tes
```powershell
cd backend
go test ./...                                                   # unit test (tanpa DB)
$env:TEST_DATABASE_URL="postgres://loyalty:loyalty@localhost:5433/loyalty?sslmode=disable"
go test ./internal/httpapi -run TestAdminFlow -v                # integrasi, butuh Postgres
```

## Struktur
- `backend/` — Go Fiber API (`cmd/api`, `internal/*`, migrasi di `db/migrations`, `sqlc.yaml`)
- `web/` — SvelteKit SPA + shadcn-svelte, tema brand di `src/app.css`

## Catatan
- Node ≥ 22.17 disarankan (SvelteKit 3 meminta itu); dev saat ini memakai 22.16 dengan `--engine-strict=false`.
- Kode QR = UUID v4 (`internal/qr`), jangan diganti ke v1/v7.

## Deploy produksi (Docker + Caddy, HTTPS otomatis)
1. Siapkan server dengan Docker, arahkan DNS domain (mis. `loyalty.masterplotter.com`) ke IP server, buka port 80 dan 443.
2. `cp .env.prod.example .env.prod` lalu isi `DOMAIN`, `DB_PASSWORD`, `JWT_SECRET` (`openssl rand -base64 48`), `ADMIN_EMAIL`, `ADMIN_PASSWORD` (min 12 karakter).
3. Jalankan:
   ```bash
   docker compose -f docker-compose.prod.yml --env-file .env.prod up -d --build
   ```
4. Login di `https://DOMAIN/admin`, lalu hapus `ADMIN_PASSWORD` dari `.env.prod` (admin hanya dibuat saat tabel `admins` kosong).

Pengaman bawaan:
- `PRODUCTION=true`: API menolak start jika `JWT_SECRET` lemah/default, `COOKIE_SECURE` mati, `PUBLIC_URL` bukan https, CORS wildcard, atau kredensial DB default.
- DB dan API tidak dibuka ke internet; hanya Caddy (80/443). Caddy menimpa `X-Forwarded-For` sehingga rate limit memakai IP klien asli dan tidak bisa dipalsukan.
- Semua respons API berisi `Cache-Control: no-store` dan header keamanan.

Backup: `docker compose -f docker-compose.prod.yml exec db pg_dump -U loyalty loyalty > backup.sql` (jadwalkan harian). Tabel `qr_codes` dan `point_ledger` adalah data kritis.

## Verifikasi nomor (OTP WhatsApp) & notifikasi
- Pembeli yang belum terverifikasi melihat kartu "Verifikasi nomor WhatsApp" di halaman Poin/Reward: kode 6 digit, berlaku 5 menit, sekali pakai.
- Batas: kirim ulang minimal 60 dtk, maks 5 permintaan/jam, maks 5 percobaan per kode (lalu terkunci). Kode disimpan sebagai HMAC, bukan teks asli.
- Pengiriman memakai **Fonnte** (`FONNTE_TOKEN`; `POST https://api.fonnte.com/send`, token tanpa Bearer, nomor dikirim sebagai `62...`). Respons `status:false` (token invalid, kuota habis, device terputus) dianggap gagal dan pembeli melihat "gagal mengirim kode". Device WhatsApp di dashboard Fonnte harus berstatus connected. Jika `FONNTE_TOKEN` kosong, fallback ke gateway generik `WA_WEBHOOK_URL`/`WA_WEBHOOK_TOKEN` (payload `{"to","message"}`).
- Dev (tanpa `WA_WEBHOOK_URL`, `PRODUCTION=false`): pesan hanya ditulis ke log API, termasuk kode OTP. Produksi tanpa gateway: OTP nonaktif (503) dan staf memverifikasi manual di Admin > Pelanggan.
- Notifikasi otomatis saat ini: kode penukaran dikirim ke nomor yang sudah terverifikasi.

## Deploy di Dokploy (dokploy.divine.co.id)
Dokploy memakai Traefik untuk domain + HTTPS, jadi pakai `docker-compose.dokploy.yml` (bukan `docker-compose.prod.yml`): tidak ada port yang dipublikasikan, Caddy hanya melayani HTTP internal (`SITE_ADDRESS=:80`).

1. **Push repo ke Git** (GitHub/GitLab/Gitea). Proyek ini belum berupa repo git: `git init`, commit, push. `.gitignore` sudah ada (`.env*` tidak ikut).
2. Di Dokploy: **Create Project → Create Service → Compose**.
   - Provider: Git (pilih repo + branch), **Compose Type: Docker Compose**, **Compose Path: `./docker-compose.dokploy.yml`**.
3. Tab **Environment**:
   ```
   DOMAIN=loyalty.masterplotter.com        # domain publik (A record -> IP server Dokploy)
   DB_PASSWORD=<acak>
   JWT_SECRET=<openssl rand -base64 48>
   ADMIN_EMAIL=owner@masterplotter.com
   ADMIN_PASSWORD=<min 12 karakter>
   FONNTE_TOKEN=<token device Fonnte>
   ```
4. Tab **Domains → Add Domain**: Host = `DOMAIN` yang sama, **Service Name = `web`**, **Container Port = `80`**, HTTPS on, Certificate = Let's Encrypt.
5. **Deploy**. Cek `https://DOMAIN/api/v1/health` → `{"status":"ok"}`, lalu login di `https://DOMAIN/admin` dan hapus `ADMIN_PASSWORD` dari Environment (hanya dipakai saat tabel admins kosong).
6. Jadwalkan backup volume `pgdata` (Dokploy: Backups) atau `pg_dump` harian.

Catatan:
- `DB_PASSWORD` hanya berlaku saat volume DB pertama kali dibuat; mengubahnya kemudian tidak mengubah password di DB yang sudah ada.
- `PUBLIC_URL` (isi QR di stiker) = `https://DOMAIN`. Jangan ganti domain setelah stiker dicetak, karena QR yang sudah ditempel menunjuk ke domain itu.
- Compose menyambungkan `web` ke network `dokploy-network` (sudah ada di server Dokploy) agar Traefik bisa menjangkaunya.
