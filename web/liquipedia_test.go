package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"pickems-bot/app"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// signallingStore reports each results fetch, so tests can wait for the
// webhook's async pipeline.
type signallingStore struct {
	*app.MockStore
	fetched chan string
}

func (s *signallingStore) FetchAndSaveMatchResults(ctx context.Context, tournamentID int, round string) error {
	err := s.MockStore.FetchAndSaveMatchResults(ctx, tournamentID, round)
	s.fetched <- round
	return err
}

func newWebhookServer(t *testing.T) (*Server, *signallingStore) {
	t.Helper()
	st := &signallingStore{MockStore: app.NewMockStore("swiss", "Stage 1"), fetched: make(chan string, 1)}
	return &Server{api: app.NewTestApp(st), page: "IEM/2026/Cologne", tournamentID: 7, round: "Stage 1"}, st
}

func postWebhook(s *Server, method, body string) int {
	rec := httptest.NewRecorder()
	s.LiquipediaWebhookHandler(rec, httptest.NewRequest(method, "/webhooks/liquipedia", strings.NewReader(body)))
	return rec.Code
}

func TestLiquipediaWebhook_RelevantPage_RunsUpdatePipeline(t *testing.T) {
	for _, page := range []string{"IEM/2026/Cologne", "IEM/2026/Cologne/Stage_1"} {
		t.Run(page, func(t *testing.T) {
			s, st := newWebhookServer(t)

			code := postWebhook(s, http.MethodPost, `{"wiki":"counterstrike","page":"`+page+`","event":"edit"}`)
			require.Equal(t, http.StatusOK, code)

			select {
			case round := <-st.fetched:
				assert.Equal(t, "Stage 1", round)
			case <-time.After(2 * time.Second):
				t.Fatal("expected the webhook to trigger a results fetch")
			}
		})
	}
}

func TestLiquipediaWebhook_IgnoredEvents(t *testing.T) {
	tests := map[string]string{
		"other wiki":                 `{"wiki":"dota2","page":"IEM/2026/Cologne"}`,
		"other page":                 `{"wiki":"counterstrike","page":"PGL/2026/Bucharest"}`,
		"page sharing a prefix only": `{"wiki":"counterstrike","page":"IEM/2026/Cologne_Qualifier"}`,
	}
	for name, body := range tests {
		t.Run(name, func(t *testing.T) {
			s, st := newWebhookServer(t)

			assert.Equal(t, http.StatusOK, postWebhook(s, http.MethodPost, body))

			select {
			case <-st.fetched:
				t.Fatal("expected no update for an irrelevant event")
			case <-time.After(50 * time.Millisecond):
			}
		})
	}
}

func TestLiquipediaWebhook_RejectsBadRequests(t *testing.T) {
	s, _ := newWebhookServer(t)

	assert.Equal(t, http.StatusMethodNotAllowed, postWebhook(s, http.MethodGet, ""))
	assert.Equal(t, http.StatusBadRequest, postWebhook(s, http.MethodPost, "not json"))
}
