//go:build integration

// Package testhelpers provides shared utilities for integration tests.
// All helpers in this package require the test server to be running on
// http://localhost:8081 before any integration test is invoked.
package testhelpers

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const TestServerBase = "http://localhost:8081"

// SetState calls the test-control endpoint to change what the mock data
// endpoint for the given source returns.
//
// source: "liquipedia" or "pandascore"
// state:  "not_started" | "ongoing" | "complete"
func SetState(t *testing.T, source, state string) {
	t.Helper()
	body := fmt.Sprintf(`{"source":%q,"state":%q}`, source, state)
	resp, err := http.Post(
		TestServerBase+"/test/state",
		"application/json",
		strings.NewReader(body),
	)
	require.NoError(t, err, "SetState: HTTP request failed")
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode,
		"SetState: expected 200 from /test/state, got %d", resp.StatusCode)
}

// MongoURI returns the MongoDB URI for integration tests. It checks
// MONGO_TEST_URI first, then MONGO_PROD_URI (the same URI the bot uses in
// production — the name is misleading; both environments share one cluster).
// If neither is set, or the server is not reachable within 2 seconds, the test
// is skipped.
func MongoURI(t *testing.T) string {
	t.Helper()
	uri := os.Getenv("MONGO_TEST_URI")
	if uri == "" {
		uri = os.Getenv("MONGO_PROD_URI")
	}
	if uri == "" {
		t.Skip("skipping: neither MONGO_TEST_URI nor MONGO_PROD_URI is set")
	}

	client, err := mongo.Connect(context.Background(), options.Client().ApplyURI(uri))
	if err != nil {
		t.Skipf("skipping: could not build mongo client: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := client.Ping(ctx, nil); err != nil {
		_ = client.Disconnect(context.Background())
		t.Skipf("skipping: MongoDB not reachable at %s: %v", uri, err)
	}
	_ = client.Disconnect(context.Background())
	return uri
}

// FireCallback asks the test server to POST a Liquipedia webhook payload to
// callbackURL after delay seconds. Returns immediately (server responds 202).
//
// page must match the page configured in the bot's test config so the webhook
// handler does not filter it out.
func FireCallback(t *testing.T, callbackURL string, delay int, page string) {
	t.Helper()
	payload := fmt.Sprintf(`{
		"callback_url": %q,
		"delay": %d,
		"payload": {"wiki":"counterstrike","page":%q,"event":"match_update"}
	}`, callbackURL, delay, page)
	resp, err := http.Post(
		TestServerBase+"/test/fire-callback",
		"application/json",
		strings.NewReader(payload),
	)
	require.NoError(t, err, "FireCallback: HTTP request failed")
	defer resp.Body.Close()
	require.Equal(t, http.StatusAccepted, resp.StatusCode,
		"FireCallback: expected 202 from /test/fire-callback, got %d", resp.StatusCode)
}
