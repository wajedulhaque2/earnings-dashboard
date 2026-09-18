package yahoo

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestUniverseUsesProviderAttributes(t *testing.T) {
	c := fixture(t, `{"finance":{"result":[{"total":2,"quotes":[{"symbol":"TEST","longName":"Operating Company","exchange":"NYQ","quoteType":"EQUITY","currency":"USD","marketCap":2000000000},{"symbol":"OTCF","exchange":"PNK","quoteType":"EQUITY","marketCap":3000000000}]}]}}`)
	rows, err := c.Universe(context.Background())
	if err != nil || len(rows) != 2 || *rows[0].ExchangeCode != "NYQ" || rows[1].UniverseEligible {
		t.Fatal(rows, err)
	}
}

func TestAuthenticationReusesCrumb(t *testing.T) {
	handshakes := 0
	unauthorized := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/cookie":
			w.WriteHeader(200)
		case "/v1/test/getcrumb":
			handshakes++
			_, _ = w.Write([]byte("fixture-crumb"))
		default:
			if r.URL.Query().Get("crumb") != "fixture-crumb" {
				unauthorized++
				w.WriteHeader(401)
				return
			}
			_, _ = w.Write([]byte(`{"quotes":[]}`))
		}
	}))
	defer srv.Close()
	c := New()
	c.Base = srv.URL
	c.AuthBase = srv.URL
	c.CookieURL = srv.URL + "/cookie"
	c.HTTP.Interval = 0
	for i := 0; i < 3; i++ {
		if _, err := c.Search(context.Background(), "TEST"); err != nil {
			t.Fatal(err)
		}
	}
	if handshakes != 1 || unauthorized != 1 {
		t.Fatal(handshakes, unauthorized)
	}
}
