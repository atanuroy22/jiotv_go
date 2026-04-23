package handlers

import "testing"

func TestIsTrustedPlaybackOrigin(t *testing.T) {
	tests := []struct {
		name   string
		scheme string
		host   string
		want   bool
	}{
		{
			name:   "HTTP localhost is trusted",
			scheme: "http",
			host:   "localhost:5001",
			want:   true,
		},
		{
			name:   "HTTP loopback IP is trusted",
			scheme: "http",
			host:   "127.0.0.1:5001",
			want:   true,
		},
		{
			name:   "HTTP LAN IP is not trusted",
			scheme: "http",
			host:   "192.168.0.101:5001",
			want:   false,
		},
		{
			name:   "HTTPS LAN IP is trusted",
			scheme: "https",
			host:   "192.168.0.101:5001",
			want:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := createMockFiberContext("GET", "/play/123")
			c.Request().URI().SetScheme(tt.scheme)
				c.Request().URI().SetHost(tt.host)
			c.Request().Header.SetHost(tt.host)
			c.Request().Header.Set("X-Forwarded-Proto", tt.scheme)

			if got := isTrustedPlaybackOrigin(c); got != tt.want {
				t.Fatalf("isTrustedPlaybackOrigin() = %v, want %v", got, tt.want)
			}
		})
	}
}
