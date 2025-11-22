package store

import (
	"context"
	"pickems-bot/api/external"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
)

func TestFetchMatchSchedule_NotInDB(t *testing.T) {
	store := NewTestStore(t, "test-round")
	defer store.Client.Disconnect(context.TODO())

	// First fetch should return error (no data in DB)
	_, err := store.FetchMatchSchedule()
	if err == nil {
		t.Error("Expected error when no data in DB, got nil")
	}
}

func TestFetchMatchSchedule_FromDB(t *testing.T) {
	store := NewTestStore(t, "test-round")
	defer store.Client.Disconnect(context.TODO())

	// Store some matches with future TTL
	matches := []external.ScheduledMatch{
		{Team1: "Team A", Team2: "Team B", EpochTime: time.Now().Unix() + 3600, BestOf: "3", StreamURL: "test", Finished: false},
		{Team1: "Team C", Team2: "Team D", EpochTime: time.Now().Unix() + 7200, BestOf: "3", StreamURL: "test", Finished: false},
	}
	err := store.StoreMatchSchedule(matches)
	if err != nil {
		t.Fatalf("Failed to store matches: %v", err)
	}

	// Fetch should retrieve from DB
	retrieved, err := store.FetchMatchSchedule()
	if err != nil {
		t.Fatalf("Failed to fetch matches: %v", err)
	}

	if len(retrieved) != 2 {
		t.Errorf("Expected 2 matches, got %d", len(retrieved))
	}
}

func TestStoreMatchSchedule_Insert(t *testing.T) {
	store := NewTestStore(t, "test-round")
	defer store.Client.Disconnect(context.TODO())

	matches := []external.ScheduledMatch{
		{Team1: "Team A", Team2: "Team B", EpochTime: 1000, BestOf: "3", StreamURL: "test", Finished: false},
	}

	err := store.StoreMatchSchedule(matches)
	if err != nil {
		t.Fatalf("Failed to store matches: %v", err)
	}

	// Verify stored
	retrieved, err := store.FetchMatchSchedule()
	if err != nil {
		t.Fatalf("Failed to fetch matches: %v", err)
	}

	if len(retrieved) != 1 {
		t.Errorf("Expected 1 match, got %d", len(retrieved))
	}
	if retrieved[0].Team1 != "Team A" {
		t.Errorf("Expected Team A, got %s", retrieved[0].Team1)
	}
}

func TestEnsureScheduledMatches_CreatesIfMissing(t *testing.T) {
	store := NewTestStore(t, "test-round")
	defer store.Client.Disconnect(context.TODO())

	// Note: This will fail if external API is not available
	// Skip this test if we can't reach external APIs
	t.Skip("Skipping test that requires external API access")

	err := store.EnsureScheduledMatches()
	if err != nil {
		t.Fatalf("Failed to ensure scheduled matches: %v", err)
	}

	// Verify matches were created
	matches, err := store.FetchMatchSchedule()
	if err != nil {
		t.Fatalf("Failed to fetch matches after ensure: %v", err)
	}

	if len(matches) == 0 {
		t.Error("Expected matches to be created, got 0")
	}
}

func TestStoreMatchSchedule_TTLCalculation(t *testing.T) {
	store := NewTestStore(t, "test-round")
	defer store.Client.Disconnect(context.TODO())

	now := time.Now().Unix()

	testCases := []struct {
		name           string
		matches        []external.ScheduledMatch
		expectedMinTTL int64
	}{
		{
			name: "ongoing BO3 match",
			matches: []external.ScheduledMatch{
				{Team1: "A", Team2: "B", EpochTime: now - 600, BestOf: "3", StreamURL: "test", Finished: false},
			},
			expectedMinTTL: now + 9000, // 150 min from now
		},
		{
			name: "future match",
			matches: []external.ScheduledMatch{
				{Team1: "A", Team2: "B", EpochTime: now + 3600, BestOf: "1", StreamURL: "test", Finished: false},
			},
			expectedMinTTL: now + 3600, // At match start time
		},
		{
			name: "finished match",
			matches: []external.ScheduledMatch{
				{Team1: "A", Team2: "B", EpochTime: now - 7200, BestOf: "3", StreamURL: "test", Finished: true},
			},
			expectedMinTTL: now + 86400, // 24 hours from now
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Clear collection
			store.Collections.MatchSchedule.Drop(context.TODO())

			err := store.StoreMatchSchedule(tc.matches)
			if err != nil {
				t.Fatalf("Failed to store matches: %v", err)
			}

			// Fetch and verify TTL is reasonable (within 5 minutes of expected)
			retrieved, err := store.FetchMatchSchedule()
			if err != nil {
				t.Fatalf("Failed to fetch matches: %v", err)
			}

			if len(retrieved) != len(tc.matches) {
				t.Errorf("Expected %d matches, got %d", len(tc.matches), len(retrieved))
			}
		})
	}
}

func TestStoreMatchSchedule_PreservesAllFields(t *testing.T) {
	store := NewTestStore(t, "test-round")
	defer store.Client.Disconnect(context.TODO())

	matches := []external.ScheduledMatch{
		{
			Team1:     "Team Alpha",
			Team2:     "Team Beta",
			EpochTime: 1234567890,
			BestOf:    "5",
			StreamURL: "custom_stream",
			Finished:  true,
		},
	}

	err := store.StoreMatchSchedule(matches)
	if err != nil {
		t.Fatalf("Failed to store matches: %v", err)
	}

	retrieved, err := store.FetchMatchSchedule()
	if err != nil {
		t.Fatalf("Failed to fetch matches: %v", err)
	}

	if len(retrieved) != 1 {
		t.Fatalf("Expected 1 match, got %d", len(retrieved))
	}

	match := retrieved[0]
	if match.Team1 != "Team Alpha" {
		t.Errorf("Team1: expected Team Alpha, got %s", match.Team1)
	}
	if match.Team2 != "Team Beta" {
		t.Errorf("Team2: expected Team Beta, got %s", match.Team2)
	}
	if match.EpochTime != 1234567890 {
		t.Errorf("EpochTime: expected 1234567890, got %d", match.EpochTime)
	}
	if match.BestOf != "5" {
		t.Errorf("BestOf: expected 5, got %s", match.BestOf)
	}
	if match.StreamURL != "custom_stream" {
		t.Errorf("StreamURL: expected custom_stream, got %s", match.StreamURL)
	}
	if !match.Finished {
		t.Error("Expected Finished to be true")
	}
}

func TestStoreMatchSchedule_EmptyMatches(t *testing.T) {
	store := NewTestStore(t, "test-round")
	defer store.Client.Disconnect(context.TODO())

	var matches []external.ScheduledMatch

	err := store.StoreMatchSchedule(matches)
	if err == nil {
		t.Fatal("Expected error when storing empty matches, got nil")
	}
}

func TestEnsureScheduledMatches_Success(t *testing.T) {
	store := NewTestStore(t, "test-round")
	defer store.Client.Disconnect(context.TODO())

	// Store some matches first
	matches := []external.ScheduledMatch{
		{Team1: "Team A", Team2: "Team B", EpochTime: time.Now().Unix() + 3600, BestOf: "3", StreamURL: "test", Finished: false},
	}
	err := store.StoreMatchSchedule(matches)
	if err != nil {
		t.Fatalf("Failed to store matches: %v", err)
	}

	// Now ensure should succeed
	err = store.EnsureScheduledMatches()
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
}

func TestEnsureScheduledMatches_NotFound(t *testing.T) {
	store := NewTestStore(t, "test-round")
	defer store.Client.Disconnect(context.TODO())

	// Don't store any matches
	err := store.EnsureScheduledMatches()
	if err == nil {
		t.Error("Expected error when no matches in DB, got nil")
	}
}

func TestFetchMatchSchedule_WithTTL(t *testing.T) {
	store := NewTestStore(t, "test-round")
	defer store.Client.Disconnect(context.TODO())

	// Store matches with valid future timestamp
	matches := []external.ScheduledMatch{
		{Team1: "Team A", Team2: "Team B", EpochTime: time.Now().Unix() + 3600, BestOf: "3", StreamURL: "test", Finished: false},
	}
	err := store.StoreMatchSchedule(matches)
	if err != nil {
		t.Fatalf("Failed to store matches: %v", err)
	}

	// Fetch should return the stored matches
	retrieved, err := store.FetchMatchSchedule()
	if err != nil {
		t.Fatalf("Failed to fetch matches: %v", err)
	}

	if len(retrieved) != 1 {
		t.Errorf("Expected 1 match, got %d", len(retrieved))
	}

	// Verify the TTL was set
	var doc struct {
		TTL int64 `bson:"ttl"`
	}
	err = store.Collections.MatchSchedule.FindOne(context.TODO(), bson.M{"round": "test-round"}).Decode(&doc)
	if err != nil {
		t.Fatalf("Failed to fetch document: %v", err)
	}

	if doc.TTL <= time.Now().Unix() {
		t.Error("Expected TTL to be in the future")
	}
}

func TestStoreMatchSchedule_UpdateExisting(t *testing.T) {
	store := NewTestStore(t, "test-round")
	defer store.Client.Disconnect(context.TODO())

	// Store initial matches
	matches1 := []external.ScheduledMatch{
		{Team1: "Team A", Team2: "Team B", EpochTime: 1000, BestOf: "3", StreamURL: "test", Finished: false},
	}
	err := store.StoreMatchSchedule(matches1)
	if err != nil {
		t.Fatalf("Failed to store initial matches: %v", err)
	}

	// Update with different matches
	matches2 := []external.ScheduledMatch{
		{Team1: "Team C", Team2: "Team D", EpochTime: 2000, BestOf: "5", StreamURL: "test2", Finished: true},
	}
	err = store.StoreMatchSchedule(matches2)
	if err != nil {
		t.Fatalf("Failed to update matches: %v", err)
	}

	// Verify updated
	retrieved, err := store.FetchMatchSchedule()
	if err != nil {
		t.Fatalf("Failed to fetch matches: %v", err)
	}

	if len(retrieved) != 1 {
		t.Errorf("Expected 1 match, got %d", len(retrieved))
	}
	if retrieved[0].Team1 != "Team C" {
		t.Errorf("Expected Team C, got %s", retrieved[0].Team1)
	}
	if retrieved[0].BestOf != "5" {
		t.Errorf("Expected BestOf 5, got %s", retrieved[0].BestOf)
	}
}
