// Package testutil holds helpers shared by tests across packages.
package testutil

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

// RouteDefaultClient sends every request made through http.DefaultClient to h,
// keeping the original path and query, for fetchers with hard-coded URLs. The
// client is restored when the test ends.
func RouteDefaultClient(t *testing.T, h http.HandlerFunc) {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	target, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	orig := http.DefaultClient.Transport
	http.DefaultClient.Transport = rewriteHost{target}
	t.Cleanup(func() { http.DefaultClient.Transport = orig })
}

type rewriteHost struct{ target *url.URL }

func (rt rewriteHost) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.URL.Scheme, r.URL.Host = rt.target.Scheme, rt.target.Host
	return http.DefaultTransport.RoundTrip(r)
}
