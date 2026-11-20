// Package phone menormalkan nomor HP Indonesia ke format E.164 (+628xxxxxxxxx).
package phone

import (
	"errors"
	"strings"
)

var ErrInvalid = errors.New("nomor WhatsApp tidak valid")

// Normalize menerima 0812..., 812..., 62812..., +62 812-xxx-xxx, dsb. Hanya nomor seluler Indonesia (MVP).
func Normalize(raw string) (string, error) {
	var b strings.Builder
	for i, r := range strings.TrimSpace(raw) {
		switch {
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '+' && i == 0, r == ' ', r == '-', r == '.', r == '(', r == ')':
		default:
			return "", ErrInvalid
		}
	}
	d := b.String()
	switch {
	case strings.HasPrefix(d, "62"):
	case strings.HasPrefix(d, "0"):
		d = "62" + d[1:]
	case strings.HasPrefix(d, "8"):
		d = "62" + d
	default:
		return "", ErrInvalid
	}
	// 62 + 8xx + 7..10 digit => 11..14 digit total
	if !strings.HasPrefix(d, "628") || len(d) < 11 || len(d) > 14 {
		return "", ErrInvalid
	}
	return "+" + d, nil
}

// Mask menyamarkan nomor untuk ditampilkan: +62812****789.
func Mask(e164 string) string {
	if len(e164) < 8 {
		return e164
	}
	return e164[:6] + strings.Repeat("*", len(e164)-9) + e164[len(e164)-3:]
}
