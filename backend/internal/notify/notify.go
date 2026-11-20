// Package notify: pengiriman pesan WhatsApp (OTP & notifikasi) lewat antarmuka yang bisa diganti.
package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/rs/zerolog/log"
)

type Sender interface {
	// Send mengirim teks ke nomor E.164 (mis. +62812...).
	Send(ctx context.Context, toE164, text string) error
}

// LogSender hanya untuk dev: menulis pesan (termasuk OTP) ke log. JANGAN dipakai di produksi.
type LogSender struct{}

func (LogSender) Send(_ context.Context, to, text string) error {
	log.Info().Str("to", to).Str("text", text).Msg("[dev] pesan WhatsApp (tidak dikirim)")
	return nil
}

// WebhookSender memanggil gateway WhatsApp lewat HTTP POST JSON:
//
//	{"to":"+62812...","message":"..."}   dengan header Authorization: Bearer <token>
//
// Bentuk ini sengaja generik; sesuaikan payload di sini bila penyedia (Fonnte, Twilio, WABA/BSP) berbeda.
type WebhookSender struct {
	URL, Token string
	Client     *http.Client
}

func NewWebhook(url, token string) *WebhookSender {
	return &WebhookSender{URL: url, Token: token, Client: &http.Client{Timeout: 10 * time.Second}}
}

func (w *WebhookSender) Send(ctx context.Context, to, text string) error {
	b, _ := json.Marshal(map[string]string{"to": to, "message": text})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.URL, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if w.Token != "" {
		req.Header.Set("Authorization", "Bearer "+w.Token)
	}
	resp, err := w.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("gateway whatsapp: status %d", resp.StatusCode)
	}
	return nil
}
