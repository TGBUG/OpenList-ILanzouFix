package ilanzou

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-resty/resty/v2"

	"github.com/OpenListTeam/OpenList/v4/internal/conf"
)

// TestUserAgentOverride pins the behaviour added on top of upstream v4.2.6:
// the driver used to send base.UserAgent ("... OpenList/<version>"), a value
// upstream #1722 introduced when it merged the per-driver UAs into base. The UA
// is now a storage option, so it can be changed without rebuilding the binary.
func TestUserAgentOverride(t *testing.T) {
	// base.NewRestyClient() dereferences conf.Conf, which is nil outside a running server.
	conf.Conf = &conf.Config{}
	var seen []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Header.Get("User-Agent"))
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/getUuid"):
			_, _ = w.Write([]byte(`{"code":200,"msg":"","uuid":"test-uuid"}`))
		case strings.HasSuffix(r.URL.Path, "/user/account/map"):
			_, _ = w.Write([]byte(`{"code":200,"msg":"","map":{"userId":"42","account":"tester@example.com"}}`))
		default:
			_, _ = w.Write([]byte(`{"code":200,"msg":""}`))
		}
	}))
	defer srv.Close()

	cases := []struct {
		name string
		ua   string
		want string
	}{
		{"empty falls back to the browser fingerprint", "", defaultUserAgent},
		{"explicit value is sent verbatim", "MyCustomAgent/9.9", "MyCustomAgent/9.9"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			seen = nil
			d := &ILanZou{
				Addition: Addition{Username: "u", Password: "p", UserAgent: tc.ua},
				conf: Conf{
					base:       srv.URL,
					secret:     []byte("lanZouY-disk-app"),
					unproved:   "unproved",
					proved:     "proved",
					devVersion: "125",
					site:       "https://www.ilanzou.com",
				},
			}
			if err := d.Init(context.Background()); err != nil {
				t.Fatalf("Init() error = %v", err)
			}
			if len(seen) == 0 {
				t.Fatal("no request reached the mock server")
			}
			for _, got := range seen {
				if got != tc.want {
					t.Fatalf("request User-Agent = %q, want %q", got, tc.want)
				}
			}
			// Every client the driver uses must carry the same fingerprint.
			for name, c := range map[string]*resty.Client{
				"apiClient":  d.apiClient,
				"linkClient": d.linkClient,
				"upClient":   d.upClient,
			} {
				if got := c.Header.Get("User-Agent"); got != tc.want {
					t.Errorf("%s User-Agent = %q, want %q", name, got, tc.want)
				}
			}
		})
	}
}
