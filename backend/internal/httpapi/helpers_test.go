package httpapi

import "github.com/masterplotter/loyalty/internal/auth"

func hashForTest(pw string) (string, error) { return auth.HashPassword(pw) }
