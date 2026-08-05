/* monitoring_pool_test.go
 * Unit tests for the monitoring pool: Subscribe, Unsubscribe, Snapshot, BootstrapPool.
 * Authors: Zachary Bower
 */

package app

import (
	"fmt"
	"testing"

	"pickems-bot/store"
)

// region Subscribe

func TestSubscribe_Success_PopulatesEntryFields(t *testing.T) {
	mockStore := NewMockStore("swiss", "test_round")
	mockStore.Tournaments = []store.Tournament{
		{ID: 42, Source: "pandascore", ExternalID: "555", SeriesID: "999", Round: "Playoffs", IsFinished: false},
	}
	api := NewTestApp(mockStore)

	api.Subscribe(bg(), 42)

	entry, ok := api.MonitoringPool.Entries[42]
	if !ok {
		t.Fatal("expected tournament to be subscribed")
	}
	if entry.DBTournamentID != 42 {
		t.Errorf("expected DBTournamentID 42, got %d", entry.DBTournamentID)
	}
	if entry.PandascoreTournamentID != 555 {
		t.Errorf("expected PandascoreTournamentID 555, got %d", entry.PandascoreTournamentID)
	}
	if entry.SeriesId != 999 {
		t.Errorf("expected SeriesId 999, got %d", entry.SeriesId)
	}
	if entry.Round != "Playoffs" {
		t.Errorf("expected Round %q, got %q", "Playoffs", entry.Round)
	}
	if entry.KnownStatus == nil {
		t.Error("expected KnownStatus to be initialised, not nil")
	}
	if len(entry.KnownStatus) != 0 {
		t.Errorf("expected KnownStatus to start empty, got %v", entry.KnownStatus)
	}
	if entry.KnownScheduleKey != "" {
		t.Errorf("expected KnownScheduleKey to start empty, got %q", entry.KnownScheduleKey)
	}
}

func TestSubscribe_AlreadyTracked_NoOp(t *testing.T) {
	mockStore := NewMockStore("swiss", "test_round")
	api := NewTestApp(mockStore)

	existing := &PoolEntry{DBTournamentID: 1, KnownStatus: map[string]string{"m1": "finished"}}
	api.MonitoringPool.Entries[1] = existing

	// No Tournaments seeded - if Subscribe fell through to GetTournament this
	// would fail loudly (the mock returns "tournament not found"), which is
	// how we know the already-tracked check really did short-circuit first.
	api.Subscribe(bg(), 1)

	if api.MonitoringPool.Entries[1] != existing {
		t.Error("expected the existing entry to be left untouched, got a different pointer")
	}
	if api.MonitoringPool.Entries[1].KnownStatus["m1"] != "finished" {
		t.Error("expected existing KnownStatus to be preserved")
	}
}

func TestSubscribe_GetTournamentError_NoOp(t *testing.T) {
	mockStore := NewMockStore("swiss", "test_round")
	mockStore.GetTournamentError = fmt.Errorf("db down")
	api := NewTestApp(mockStore)

	api.Subscribe(bg(), 1)

	if _, ok := api.MonitoringPool.Entries[1]; ok {
		t.Error("expected no entry to be added when GetTournament fails")
	}
}

func TestSubscribe_NonPandaScoreSource_NoOp(t *testing.T) {
	mockStore := NewMockStore("swiss", "test_round")
	mockStore.Tournaments = []store.Tournament{
		{ID: 1, Source: "liquipedia", ExternalID: "some-page"},
	}
	api := NewTestApp(mockStore)

	api.Subscribe(bg(), 1)

	if _, ok := api.MonitoringPool.Entries[1]; ok {
		t.Error("expected a liquipedia-sourced tournament not to be subscribed")
	}
}

func TestSubscribe_FinishedTournament_NoOp(t *testing.T) {
	mockStore := NewMockStore("swiss", "test_round")
	mockStore.Tournaments = []store.Tournament{
		{ID: 1, Source: "pandascore", ExternalID: "1", SeriesID: "1", IsFinished: true},
	}
	api := NewTestApp(mockStore)

	api.Subscribe(bg(), 1)

	if _, ok := api.MonitoringPool.Entries[1]; ok {
		t.Error("expected a finished tournament not to be subscribed")
	}
}

func TestSubscribe_InvalidExternalID_NoOp(t *testing.T) {
	mockStore := NewMockStore("swiss", "test_round")
	mockStore.Tournaments = []store.Tournament{
		{ID: 1, Source: "pandascore", ExternalID: "not-a-number", SeriesID: "1"},
	}
	api := NewTestApp(mockStore)

	api.Subscribe(bg(), 1)

	if _, ok := api.MonitoringPool.Entries[1]; ok {
		t.Error("expected a tournament with a non-numeric external id not to be subscribed")
	}
}

func TestSubscribe_InvalidSeriesID_NoOp(t *testing.T) {
	mockStore := NewMockStore("swiss", "test_round")
	mockStore.Tournaments = []store.Tournament{
		{ID: 1, Source: "pandascore", ExternalID: "123", SeriesID: "not-a-number"},
	}
	api := NewTestApp(mockStore)

	api.Subscribe(bg(), 1)

	if _, ok := api.MonitoringPool.Entries[1]; ok {
		t.Error("expected a tournament with a non-numeric series id not to be subscribed")
	}
}

// endregion

// region Unsubscribe

func TestUnsubscribe_RemovesExistingEntry(t *testing.T) {
	api := NewTestApp(NewMockStore("swiss", "test_round"))
	api.MonitoringPool.Entries[1] = &PoolEntry{DBTournamentID: 1}

	api.Unsubscribe(bg(), 1)

	if _, ok := api.MonitoringPool.Entries[1]; ok {
		t.Error("expected entry to be removed")
	}
}

func TestUnsubscribe_NotTracked_NoOp(t *testing.T) {
	api := NewTestApp(NewMockStore("swiss", "test_round"))

	api.Unsubscribe(bg(), 999) // must not panic on a missing key

	if len(api.MonitoringPool.Entries) != 0 {
		t.Error("expected pool to remain empty")
	}
}

// endregion

// region Snapshot

func TestSnapshot_ReturnsAllEntries(t *testing.T) {
	api := NewTestApp(NewMockStore("swiss", "test_round"))
	api.MonitoringPool.Entries[1] = &PoolEntry{DBTournamentID: 1}
	api.MonitoringPool.Entries[2] = &PoolEntry{DBTournamentID: 2}

	snapshot := api.Snapshot()

	if len(snapshot) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(snapshot))
	}
	ids := map[int]bool{}
	for _, e := range snapshot {
		ids[e.DBTournamentID] = true
	}
	if !ids[1] || !ids[2] {
		t.Error("expected snapshot to contain both tracked tournaments")
	}
}

func TestSnapshot_EmptyPool_ReturnsEmptySlice(t *testing.T) {
	api := NewTestApp(NewMockStore("swiss", "test_round"))

	snapshot := api.Snapshot()

	if len(snapshot) != 0 {
		t.Errorf("expected empty snapshot, got %d entries", len(snapshot))
	}
}

// endregion

// region BootstrapPool

func TestBootstrapPool_SubscribesAllTrackedIDs(t *testing.T) {
	mockStore := NewMockStore("swiss", "test_round")
	mockStore.ListTrackedTournamentIDsResult = []int{1, 2}
	mockStore.Tournaments = []store.Tournament{
		{ID: 1, Source: "pandascore", ExternalID: "1", SeriesID: "1"},
		{ID: 2, Source: "pandascore", ExternalID: "2", SeriesID: "2"},
	}
	api := NewTestApp(mockStore)

	if err := api.BootstrapPool(bg()); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if _, ok := api.MonitoringPool.Entries[1]; !ok {
		t.Error("expected tournament 1 to be subscribed")
	}
	if _, ok := api.MonitoringPool.Entries[2]; !ok {
		t.Error("expected tournament 2 to be subscribed")
	}
}

func TestBootstrapPool_SkipsFinishedAndWrongSource(t *testing.T) {
	mockStore := NewMockStore("swiss", "test_round")
	mockStore.ListTrackedTournamentIDsResult = []int{1, 2, 3}
	mockStore.Tournaments = []store.Tournament{
		{ID: 1, Source: "pandascore", ExternalID: "1", SeriesID: "1"},
		{ID: 2, Source: "pandascore", ExternalID: "2", SeriesID: "2", IsFinished: true},
		{ID: 3, Source: "liquipedia", ExternalID: "page-3"},
	}
	api := NewTestApp(mockStore)

	if err := api.BootstrapPool(bg()); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if _, ok := api.MonitoringPool.Entries[1]; !ok {
		t.Error("expected the live pandascore tournament to be subscribed")
	}
	if _, ok := api.MonitoringPool.Entries[2]; ok {
		t.Error("expected the finished tournament to be skipped")
	}
	if _, ok := api.MonitoringPool.Entries[3]; ok {
		t.Error("expected the liquipedia tournament to be skipped")
	}
}

func TestBootstrapPool_ListError_Propagates(t *testing.T) {
	mockStore := NewMockStore("swiss", "test_round")
	mockStore.ListTrackedTournamentIDsError = fmt.Errorf("db down")
	api := NewTestApp(mockStore)

	if err := api.BootstrapPool(bg()); err == nil {
		t.Error("expected error to propagate from ListTrackedTournamentIDs")
	}
}

// endregion

// region updatePool

func TestUpdatePool_NoPreviousTournament_SubscribesNew(t *testing.T) {
	mockStore := NewMockStore("swiss", "test_round")
	mockStore.Tournaments = []store.Tournament{
		{ID: 2, Source: "pandascore", ExternalID: "2", SeriesID: "2"},
	}
	api := NewTestApp(mockStore)

	api.updatePool(bg(), store.GuildConfig{}, 2) // first-time config: no previous tournament

	if _, ok := api.MonitoringPool.Entries[2]; !ok {
		t.Error("expected the new tournament to be subscribed")
	}
}

func TestUpdatePool_SameTournament_LeavesExistingEntryInPlace(t *testing.T) {
	api := NewTestApp(NewMockStore("swiss", "test_round"))

	existing := &PoolEntry{DBTournamentID: 5, KnownStatus: map[string]string{"m1": "finished"}}
	api.MonitoringPool.Entries[5] = existing

	oldID := 5
	prev := store.GuildConfig{ID: 10, TournamentID: &oldID}
	api.updatePool(bg(), prev, 5) // re-confirming the same tournament, e.g. re-running /config

	if api.MonitoringPool.Entries[5] != existing {
		t.Error("expected the existing entry to be left untouched when old and new tournament ids match")
	}
}

func TestUpdatePool_DifferentTournament_StillReferenced_KeepsOldSubscribed(t *testing.T) {
	mockStore := NewMockStore("swiss", "test_round")
	mockStore.Tournaments = []store.Tournament{
		{ID: 2, Source: "pandascore", ExternalID: "2", SeriesID: "2"},
	}
	mockStore.TournamentStillReferencedResult = true
	api := NewTestApp(mockStore)

	oldID := 1
	api.MonitoringPool.Entries[oldID] = &PoolEntry{DBTournamentID: oldID}
	prev := store.GuildConfig{ID: 10, TournamentID: &oldID}

	api.updatePool(bg(), prev, 2)

	if _, ok := api.MonitoringPool.Entries[oldID]; !ok {
		t.Error("expected the old tournament to stay subscribed - another guild still references it")
	}
	if _, ok := api.MonitoringPool.Entries[2]; !ok {
		t.Error("expected the new tournament to be subscribed")
	}
}

func TestUpdatePool_DifferentTournament_NotReferenced_UnsubscribesOld(t *testing.T) {
	mockStore := NewMockStore("swiss", "test_round")
	mockStore.Tournaments = []store.Tournament{
		{ID: 2, Source: "pandascore", ExternalID: "2", SeriesID: "2"},
	}
	mockStore.TournamentStillReferencedResult = false
	api := NewTestApp(mockStore)

	oldID := 1
	api.MonitoringPool.Entries[oldID] = &PoolEntry{DBTournamentID: oldID}
	prev := store.GuildConfig{ID: 10, TournamentID: &oldID}

	api.updatePool(bg(), prev, 2)

	if _, ok := api.MonitoringPool.Entries[oldID]; ok {
		t.Error("expected the old tournament to be unsubscribed - nothing else references it")
	}
	if _, ok := api.MonitoringPool.Entries[2]; !ok {
		t.Error("expected the new tournament to be subscribed")
	}
}

func TestUpdatePool_TournamentStillReferencedError_LeavesOldSubscribed(t *testing.T) {
	mockStore := NewMockStore("swiss", "test_round")
	mockStore.Tournaments = []store.Tournament{
		{ID: 2, Source: "pandascore", ExternalID: "2", SeriesID: "2"},
	}
	mockStore.TournamentStillReferencedError = fmt.Errorf("db down")
	api := NewTestApp(mockStore)

	oldID := 1
	api.MonitoringPool.Entries[oldID] = &PoolEntry{DBTournamentID: oldID}
	prev := store.GuildConfig{ID: 10, TournamentID: &oldID}

	api.updatePool(bg(), prev, 2)

	if _, ok := api.MonitoringPool.Entries[oldID]; !ok {
		t.Error("expected the old tournament to stay subscribed when the referenced-check itself errors")
	}
	if _, ok := api.MonitoringPool.Entries[2]; !ok {
		t.Error("expected the new tournament to be subscribed regardless")
	}
}

// endregion
