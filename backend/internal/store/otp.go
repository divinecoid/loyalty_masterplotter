package store

import (
	"context"
	"crypto/hmac"
	"errors"
	"time"
)

var (
	ErrOTPTooSoon  = errors.New("otp: tunggu sebelum meminta kode baru")
	ErrOTPTooMany  = errors.New("otp: terlalu banyak permintaan")
	ErrOTPInvalid  = errors.New("otp: kode salah atau kedaluwarsa")
	ErrOTPLocked   = errors.New("otp: terlalu banyak percobaan")
	otpCooldown    = 60 * time.Second
	otpMaxPerHour  = 5
	otpMaxAttempts = 5
)

// CreateOTP menyimpan hash kode baru dan membatalkan kode sebelumnya. Dibatasi cooldown & kuota per jam.
func (s *Store) CreateOTP(ctx context.Context, customerID int64, codeHash []byte, ttl time.Duration) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	if _, err := tx.Exec(ctx, `SELECT 1 FROM customers WHERE id=$1 FOR UPDATE`, customerID); err != nil {
		return err
	}
	var recent, hour int
	if err := tx.QueryRow(ctx,
		`SELECT count(*) FILTER (WHERE created_at > now() - make_interval(secs => $2)),
		        count(*) FILTER (WHERE created_at > now() - interval '1 hour')
		 FROM otp_codes WHERE customer_id=$1`, customerID, otpCooldown.Seconds()).Scan(&recent, &hour); err != nil {
		return err
	}
	if recent > 0 {
		return ErrOTPTooSoon
	}
	if hour >= otpMaxPerHour {
		return ErrOTPTooMany
	}
	if _, err := tx.Exec(ctx, `UPDATE otp_codes SET consumed_at=now() WHERE customer_id=$1 AND consumed_at IS NULL`, customerID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO otp_codes(customer_id,code_hash,expires_at) VALUES($1,$2,now()+make_interval(secs => $3))`,
		customerID, codeHash, ttl.Seconds()); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// VerifyOTP: cocokkan hash dengan kode aktif terbaru. Percobaan salah dihitung (dan di-commit);
// setelah otpMaxAttempts kode dikunci. Berhasil -> kode dikonsumsi dan nomor ditandai terverifikasi.
func (s *Store) VerifyOTP(ctx context.Context, customerID int64, codeHash []byte) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	if _, err := tx.Exec(ctx, `SELECT 1 FROM customers WHERE id=$1 FOR UPDATE`, customerID); err != nil {
		return err
	}
	var id int64
	var stored []byte
	var attempts int
	err = tx.QueryRow(ctx,
		`SELECT id,code_hash,attempts FROM otp_codes
		 WHERE customer_id=$1 AND consumed_at IS NULL AND expires_at > now()
		 ORDER BY id DESC LIMIT 1 FOR UPDATE`, customerID).Scan(&id, &stored, &attempts)
	if isNoRows(err) {
		return ErrOTPInvalid
	}
	if err != nil {
		return err
	}
	if attempts >= otpMaxAttempts {
		return ErrOTPLocked
	}
	if !hmac.Equal(stored, codeHash) {
		if _, err := tx.Exec(ctx, `UPDATE otp_codes SET attempts=attempts+1 WHERE id=$1`, id); err != nil {
			return err
		}
		if err := tx.Commit(ctx); err != nil {
			return err
		}
		if attempts+1 >= otpMaxAttempts {
			return ErrOTPLocked
		}
		return ErrOTPInvalid
	}
	if _, err := tx.Exec(ctx, `UPDATE otp_codes SET consumed_at=now() WHERE id=$1`, id); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE customers SET phone_verified=true WHERE id=$1`, customerID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
