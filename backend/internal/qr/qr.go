package qr

import (
	"strings"

	"github.com/google/uuid"
)

// NewCode membuat kode QR: UUID v4 dari CSPRNG (122 bit acak, tidak bisa ditebak).
// Sengaja BUKAN v1/v7 karena mengandung komponen waktu/urutan.
func NewCode() uuid.UUID { return uuid.New() }

// URL membangun isi QR: {publicURL}/s/{uuid}
func URL(publicURL string, code uuid.UUID) string {
	return strings.TrimRight(publicURL, "/") + "/s/" + code.String()
}
