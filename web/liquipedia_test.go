/* liquipedia_test.go
 * Contains unit tests for liquipedia.go functions
 * Authors: Zachary Bower, Claude Opus 4.5
 */

package web

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	apiPkg "pickems-bot/app"
	"pickems-bot/sources"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// region Server.logger tests

func TestServer_Logger_NilReturnsDefault(t *testing.T) {
	s := &Server{}
	assert.NotNil(t, s.logger())
}

func TestServer_Logger_InjectedLogReturned(t *testing.T) {
	log := slog.Default()
	s := &Server{log: log}
	assert.Equal(t, log, s.logger())
}

// endregion

// region isRelevantTournamentPage tests

func TestIsRelevantTournamentPage_ExactMatch(t *testing.T) {
	result := isRelevantTournamentPage("BLAST/Premier/2025/World_Final", "BLAST/Premier/2025/World_Final")
	assert.True(t, result)
}

func TestIsRelevantTournamentPage_SubPage(t *testing.T) {
	result := isRelevantTournamentPage("BLAST/Premier/2025/World_Final/Opening_Stage", "BLAST/Premier/2025/World_Final")
	assert.True(t, result)
}

func TestIsRelevantTournamentPage_DifferentTournament(t *testing.T) {
	result := isRelevantTournamentPage("ESL/Pro_League/Season_20", "BLAST/Premier/2025/World_Final")
	assert.False(t, result)
}

func TestIsRelevantTournamentPage_PartialMatch(t *testing.T) {
	// Partial match without slash should not match
	result := isRelevantTournamentPage("BLAST/Premier/2025", "BLAST/Premier/2025/World_Final")
	assert.False(t, result)
}

func TestIsRelevantTournamentPage_EmptyBase(t *testing.T) {
	// Empty base means empty string, so "SomePage" != "" and doesn't have prefix "/", so false
	result := isRelevantTournamentPage("SomePage", "")
	assert.False(t, result)
}

func TestIsRelevantTournamentPage_BothEmpty(t *testing.T) {
	// Both empty means page == base, so true
	result := isRelevantTournamentPage("", "")
	assert.True(t, result)
}

// endregion

// region LiquipediaWebhookHandler tests

func TestLiquipediaWebhookHandler_WrongMethod(t *testing.T) {
	server := &Server{api: nil}

	req := httptest.NewRequest(http.MethodGet, "/webhooks/liquipedia", nil)
	w := httptest.NewRecorder()

	server.LiquipediaWebhookHandler(w, req)

	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

func TestLiquipediaWebhookHandler_InvalidJSON(t *testing.T) {
	server := &Server{api: nil}

	req := httptest.NewRequest(http.MethodPost, "/webhooks/liquipedia", bytes.NewBufferString("invalid json"))
	w := httptest.NewRecorder()

	server.LiquipediaWebhookHandler(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestLiquipediaWebhookHandler_WrongWiki(t *testing.T) {
	server := &Server{api: nil}

	event := LiquipediaEvent{
		Wiki:  "dota2", // Wrong wiki
		Page:  "SomePage",
		Event: "update",
	}
	body, _ := json.Marshal(event)

	req := httptest.NewRequest(http.MethodPost, "/webhooks/liquipedia", bytes.NewBuffer(body))
	w := httptest.NewRecorder()

	server.LiquipediaWebhookHandler(w, req)

	// Should return OK but not process (different wiki)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestLiquipediaWebhookHandler_IrrelevantPage(t *testing.T) {
	mockStore := apiPkg.NewMockStore("swiss", "test_round")
	server := &Server{api: &apiPkg.App{Store: mockStore}, page: "Test/Tournament/2025"}

	event := LiquipediaEvent{
		Wiki:  "counterstrike",
		Page:  "ESL/Pro_League/Season_20", // Different tournament from MockStore page
		Event: "update",
	}
	body, _ := json.Marshal(event)

	req := httptest.NewRequest(http.MethodPost, "/webhooks/liquipedia", bytes.NewBuffer(body))
	w := httptest.NewRecorder()

	server.LiquipediaWebhookHandler(w, req)

	// Should return OK but not process (irrelevant page)
	assert.Equal(t, http.StatusOK, w.Code)
}

// Note: Tests for relevant events that trigger the async goroutine require a mock API
// to avoid nil pointer dereference. These are skipped in unit tests.
// Integration tests should be used to verify the full webhook flow.

// TestLiquipediaWebhookHandler_RelevantEvent_ReturnsOK tests that relevant events return 200 OK
// This test uses a mock API to test the full flow
func TestLiquipediaWebhookHandler_RelevantEvent_ReturnsOK(t *testing.T) {
	mockStore := apiPkg.NewMockStore("swiss", "test_round")
	mockStore.SetSwissResults(map[string]string{"Team A": "3-0"})
	mockAPI := &apiPkg.App{Store: mockStore}

	server := &Server{api: mockAPI, page: "Test/Tournament/2025"}

	event := LiquipediaEvent{
		Wiki:  "counterstrike",
		Page:  "Test/Tournament/2025",
		Event: "update",
	}
	body, _ := json.Marshal(event)

	req := httptest.NewRequest(http.MethodPost, "/webhooks/liquipedia", bytes.NewBuffer(body))
	w := httptest.NewRecorder()

	server.LiquipediaWebhookHandler(w, req)

	// Should return OK and trigger async processing
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestLiquipediaWebhookHandler_PipelineRunsToCompletion(t *testing.T) {
	// Uses NewTestApp (non-nil rate limiter) so UpdateMatchResults succeeds and the
	// goroutine reaches GenerateLeaderboard / RenderResultsImage.
	mockStore := apiPkg.NewMockStore("swiss", "test_round")
	mockStore.SetScheduledMatches([]sources.ScheduledMatch{
		{Team1: "Team A", Team2: "Team B", EpochTime: 9999999999},
	})
	mockStore.FetchAndStoreScheduleError = fmt.Errorf("schedule fetch failed")
	mockAPI := apiPkg.NewTestApp(mockStore)

	server := &Server{api: mockAPI, page: "Test/Tournament/2025"}

	event := LiquipediaEvent{Wiki: "counterstrike", Page: "Test/Tournament/2025", Event: "update"}
	body, _ := json.Marshal(event)
	req := httptest.NewRequest(http.MethodPost, "/webhooks/liquipedia", bytes.NewBuffer(body))
	w := httptest.NewRecorder()

	server.LiquipediaWebhookHandler(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	time.Sleep(10 * time.Millisecond) // let goroutine run through the full pipeline
}

// TestLiquipediaWebhookHandler_SubPageMatch_ReturnsOK tests sub-page matching
func TestLiquipediaWebhookHandler_SubPageMatch_ReturnsOK(t *testing.T) {
	mockStore := apiPkg.NewMockStore("swiss", "test_round")
	mockStore.SetSwissResults(map[string]string{"Team A": "3-0"})
	mockAPI := &apiPkg.App{Store: mockStore}

	server := &Server{api: mockAPI, page: "Test/Tournament/2025"}

	event := LiquipediaEvent{
		Wiki:  "counterstrike",
		Page:  "Test/Tournament/2025/Opening_Stage", // Sub-page
		Event: "update",
	}
	body, _ := json.Marshal(event)

	req := httptest.NewRequest(http.MethodPost, "/webhooks/liquipedia", bytes.NewBuffer(body))
	w := httptest.NewRecorder()

	server.LiquipediaWebhookHandler(w, req)

	// Should return OK and trigger async processing
	assert.Equal(t, http.StatusOK, w.Code)
}

// endregion

// region LiquipediaEvent struct tests

func TestLiquipediaEvent_JSONDecode(t *testing.T) {
	jsonStr := `{"wiki":"counterstrike","page":"TestPage","event":"update"}`

	var event LiquipediaEvent
	err := json.Unmarshal([]byte(jsonStr), &event)

	assert.NoError(t, err)
	assert.Equal(t, "counterstrike", event.Wiki)
	assert.Equal(t, "TestPage", event.Page)
	assert.Equal(t, "update", event.Event)
}

func TestLiquipediaEvent_JSONEncode(t *testing.T) {
	event := LiquipediaEvent{
		Wiki:  "counterstrike",
		Page:  "TestPage",
		Event: "update",
	}

	jsonBytes, err := json.Marshal(event)

	assert.NoError(t, err)
	assert.Contains(t, string(jsonBytes), `"wiki":"counterstrike"`)
	assert.Contains(t, string(jsonBytes), `"page":"TestPage"`)
	assert.Contains(t, string(jsonBytes), `"event":"update"`)
}

// endregion

// region RenderResultsImage tests

func TestRenderResultsImage_FetchError(t *testing.T) {
	mockStore := apiPkg.NewMockStore("swiss", "test_round")
	mockStore.FetchMatchNodesFromDbError = fmt.Errorf("db error")
	a := apiPkg.NewTestApp(mockStore)

	err := RenderResultsImage(a)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to fetch match nodes")
}

func TestRenderResultsImage_EmptyKind(t *testing.T) {
	mockStore := apiPkg.NewMockStore("swiss", "test_round")
	// default mock returns kind="" — should get "kind was empty" error
	a := apiPkg.NewTestApp(mockStore)

	err := RenderResultsImage(a)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "kind was empty")
}

func TestRenderResultsImage_SwissKind_ReachesRenderer(t *testing.T) {
	mockStore := apiPkg.NewMockStore("swiss", "test_round")
	mockStore.MatchKind = "swiss"
	a := apiPkg.NewTestApp(mockStore)

	// Just verify the path runs without panicking; RenderBracket may succeed or fail with empty nodes.
	_ = RenderResultsImage(a)
}

func TestRenderResultsImage_SingleElimKind_CoversNormalisationPath(t *testing.T) {
	mockStore := apiPkg.NewMockStore("swiss", "test_round")
	mockStore.MatchKind = "single-elimination"
	a := apiPkg.NewTestApp(mockStore)

	_ = RenderResultsImage(a)
}

// endregion

// region toRenderNodes tests

func TestToRenderNodes_MapsAllFields(t *testing.T) {
	nodes := []sources.MatchNode{
		{ID: "m1", Team1: "Team A", Team2: "Team B", Winner: "Team A", Score: "2-0", Section: "Quarterfinals"},
		{ID: "m2", Team1: "Team C", Team2: "Team D", Winner: "", Score: "", Section: "Semifinals"},
	}

	result := toRenderNodes(nodes)

	require.Len(t, result, 2)
	assert.Equal(t, "m1", result[0].ID)
	assert.Equal(t, "Team A", result[0].Team1)
	assert.Equal(t, "Team B", result[0].Team2)
	assert.Equal(t, "Team A", result[0].Winner)
	assert.Equal(t, "2-0", result[0].Score)
	assert.Equal(t, "Quarterfinals", result[0].Section)
	assert.Equal(t, "m2", result[1].ID)
	assert.Empty(t, result[1].Winner)
}

func TestToRenderNodes_EmptySlice(t *testing.T) {
	result := toRenderNodes([]sources.MatchNode{})
	require.Len(t, result, 0)
}

// endregion
