package ratelimit

import (
	"net/http"
	"testing"
)

func TestExtractIP(t *testing.T) {
	tests := []struct {
		name       string
		xff        string
		remoteAddr string
		want       string
	}{
		{
			name:       "XFF single IP",
			xff:        "1.2.3.4",
			remoteAddr: "9.9.9.9:1234",
			want:       "1.2.3.4",
		},
		{
			name:       "XFF multiple IPs takes first",
			xff:        "1.2.3.4, 5.6.7.8",
			remoteAddr: "9.9.9.9:1234",
			want:       "1.2.3.4",
		},
		{
			name:       "no XFF falls back to RemoteAddr with port",
			xff:        "",
			remoteAddr: "192.168.1.1:12345",
			want:       "192.168.1.1",
		},
		{
			name:       "IPv6 RemoteAddr with port",
			xff:        "",
			remoteAddr: "[::1]:8080",
			want:       "::1",
		},
		{
			name:       "RemoteAddr without port",
			xff:        "",
			remoteAddr: "10.0.0.1",
			want:       "10.0.0.1",
		},
		{
			name:       "XFF with port strips port",
			xff:        "1.2.3.4:9999",
			remoteAddr: "9.9.9.9:1234",
			want:       "1.2.3.4",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &http.Request{
				RemoteAddr: tt.remoteAddr,
				Header:     http.Header{},
			}
			if tt.xff != "" {
				r.Header.Set("X-Forwarded-For", tt.xff)
			}

			got := ExtractIP(r)
			if got != tt.want {
				t.Errorf("ExtractIP() = %q, want %q", got, tt.want)
			}
		})
	}
}
