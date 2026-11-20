package notify

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const fonnteURL = "https://api.fonnte.com/send"

// FonnteSender mengirim lewat https://docs.fonnte.com/api-send-message/
//   - Authorization: <token> (tanpa "Bearer")
//   - target: nomor berawalan kode negara tanpa "+", countryCode "0" agar Fonnte tidak mengubah nomor
//   - kegagalan ditandai "status": false di body (kapitalisasi bisa "Status"), alasannya di "reason"
type FonnteSender struct {
	Token  string
	URL    string // default fonnteURL; bisa diganti untuk tes
	Client *http.Client
}

func NewFonnte(token string) *FonnteSender {
	return &FonnteSender{Token: token, URL: fonnteURL, Client: &http.Client{Timeout: 10 * time.Second}}
}

func (f *FonnteSender) Send(ctx context.Context, toE164, text string) error {
	form := url.Values{
		"target":      {strings.TrimPrefix(toE164, "+")},
		"message":     {text},
		"countryCode": {"0"},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, f.URL, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", f.Token)
	resp, err := f.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	return parseFonnte(resp.StatusCode, body)
}

func parseFonnte(httpStatus int, body []byte) error {
	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		return fmt.Errorf("fonnte: respons tidak valid (http %d)", httpStatus)
	}
	ok, found := false, false
	for _, k := range []string{"status", "Status"} {
		if v, has := m[k]; has {
			ok, found = v == true, true
			break
		}
	}
	if httpStatus/100 != 2 || !found || !ok {
		reason, _ := m["reason"].(string)
		return fmt.Errorf("fonnte: gagal (http %d): %s", httpStatus, reason)
	}
	return nil
}
