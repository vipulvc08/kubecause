package pagerduty

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func TestVerifySignature(t *testing.T) {
	secret := "shhh"
	body := []byte(`{"event":{"id":"abc"}}`)

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	good := "v1=" + hex.EncodeToString(mac.Sum(nil))

	tests := []struct {
		name   string
		header string
		secret string
		want   bool
	}{
		{"valid signature", good, secret, true},
		{"valid with rotation candidate", "v1=deadbeef," + good, secret, true},
		{"bad signature", "v1=deadbeef", secret, false},
		{"empty header", "", secret, false},
		{"no secret configured (dev)", "", "", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := verifySignature(tc.header, tc.secret, body); got != tc.want {
				t.Errorf("verifySignature = %v, want %v", got, tc.want)
			}
		})
	}
}
