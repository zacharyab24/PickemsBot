/* liquipedia_test.go
 * Contains unit tests for liquipedia.go functions
 * Authors: Zachary Bower, Claude Opus 4.5
 */

package web

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	apiPkg "pickems-bot/api/api"

	"github.com/stretchr/testify/assert"
)

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
	// Set the PAGE environment variable
	originalPage := os.Getenv("PAGE")
	os.Setenv("PAGE", "BLAST/Premier/2025/World_Final")
	defer os.Setenv("PAGE", originalPage)

	server := &Server{api: nil}

	event := LiquipediaEvent{
		Wiki:  "counterstrike",
		Page:  "ESL/Pro_League/Season_20", // Different tournament
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
	// Set the PAGE environment variable
	originalPage := os.Getenv("PAGE")
	os.Setenv("PAGE", "BLAST/Premier/2025/World_Final")
	defer os.Setenv("PAGE", originalPage)

	// Create server with mock API from api package
	mockStore := apiPkg.NewMockStore("swiss", "test_round")
	mockStore.SetSwissResults(map[string]string{"Team A": "3-0"})
	mockAPI := &apiPkg.API{Store: mockStore}

	server := &Server{api: mockAPI}

	event := LiquipediaEvent{
		Wiki:  "counterstrike",
		Page:  "BLAST/Premier/2025/World_Final",
		Event: "update",
	}
	body, _ := json.Marshal(event)

	req := httptest.NewRequest(http.MethodPost, "/webhooks/liquipedia", bytes.NewBuffer(body))
	w := httptest.NewRecorder()

	server.LiquipediaWebhookHandler(w, req)

	// Should return OK and trigger async processing
	assert.Equal(t, http.StatusOK, w.Code)
}

// TestLiquipediaWebhookHandler_SubPageMatch_ReturnsOK tests sub-page matching
func TestLiquipediaWebhookHandler_SubPageMatch_ReturnsOK(t *testing.T) {
	// Set the PAGE environment variable
	originalPage := os.Getenv("PAGE")
	os.Setenv("PAGE", "BLAST/Premier/2025/World_Final")
	defer os.Setenv("PAGE", originalPage)

	// Create server with mock API
	mockStore := apiPkg.NewMockStore("swiss", "test_round")
	mockStore.SetSwissResults(map[string]string{"Team A": "3-0"})
	mockAPI := &apiPkg.API{Store: mockStore}

	server := &Server{api: mockAPI}

	event := LiquipediaEvent{
		Wiki:  "counterstrike",
		Page:  "BLAST/Premier/2025/World_Final/Opening_Stage", // Sub-page
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
