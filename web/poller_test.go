/* poller_test.go
 * Contains unit tests for poller.go functions
 * Authors: Zachary Bower
 */

package web

import (
	"log/slog"
	"pickems-bot/app"
	"pickems-bot/sources"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// region NewPoller tests

func TestNewPoller_DefaultInterval(t *testing.T) {
	p := NewPoller(nil, "test-key", "", nil)

	assert.Equal(t, time.Minute, p.interval)
}

func TestNewPoller_Fields(t *testing.T) {
	p := NewPoller(nil, "my-api-key", "", nil)

	assert.Nil(t, p.app)
	assert.Equal(t, "my-api-key", p.apiKey)
}

// endregion

// region logger tests

func TestPoller_Logger_NilLog_ReturnsDefault(t *testing.T) {
	p := NewPoller(nil, "key", "", nil)
	l := p.logger()
	assert.NotNil(t, l)
}

func TestPoller_Logger_InjectedLog(t *testing.T) {
	p := NewPoller(nil, "key", "", slog.Default())
	l := p.logger()
	assert.NotNil(t, l)
}

// endregion

// region KnownStatus transition logic tests
//
// These exercise the same transition check tick() runs inline, but against a
// PoolEntry directly - that's where KnownStatus lives now that one Poller
// tracks many tournaments instead of one.

func TestPoolEntry_StatusTransition_DetectsFinished(t *testing.T) {
	entry := &app.PoolEntry{KnownStatus: map[string]string{"match-1": "running"}}

	// Simulate a tick where match-1 transitions to finished
	finishedTransition := false
	statuses := map[string]string{"match-1": "finished"}

	for id, status := range statuses {
		prev, seen := entry.KnownStatus[id]
		if seen && prev != "finished" && status == "finished" {
			finishedTransition = true
		}
		entry.KnownStatus[id] = status
	}

	assert.True(t, finishedTransition)
	assert.Equal(t, "finished", entry.KnownStatus["match-1"])
}

func TestPoolEntry_StatusTransition_NoTriggerIfAlreadyFinished(t *testing.T) {
	// A match already marked finished should not trigger again on next tick
	entry := &app.PoolEntry{KnownStatus: map[string]string{"match-1": "finished"}}

	finishedTransition := false
	statuses := map[string]string{"match-1": "finished"}

	for id, status := range statuses {
		prev, seen := entry.KnownStatus[id]
		if seen && prev != "finished" && status == "finished" {
			finishedTransition = true
		}
		entry.KnownStatus[id] = status
	}

	assert.False(t, finishedTransition)
}

func TestPoolEntry_StatusTransition_NoTriggerForFirstSeen(t *testing.T) {
	// A brand-new match seen as "finished" (never tracked before) should not trigger —
	// we only react to transitions, not initial state.
	entry := &app.PoolEntry{KnownStatus: map[string]string{}}

	finishedTransition := false
	statuses := map[string]string{"match-new": "finished"}

	for id, status := range statuses {
		prev, seen := entry.KnownStatus[id]
		if seen && prev != "finished" && status == "finished" {
			finishedTransition = true
		}
		entry.KnownStatus[id] = status
	}

	assert.False(t, finishedTransition)
}

// endregion

// region scheduleKey tests

func TestScheduleKey_SameMatchesSameKey(t *testing.T) {
	matches := []sources.ScheduledMatch{
		{Team1: "Team A", Team2: "Team B", EpochTime: 1000},
		{Team1: "Team C", Team2: "Team D", EpochTime: 2000},
	}
	assert.Equal(t, scheduleKey(matches), scheduleKey(matches))
}

func TestScheduleKey_OrderIndependent(t *testing.T) {
	a := []sources.ScheduledMatch{
		{Team1: "Team A", Team2: "Team B", EpochTime: 1000},
		{Team1: "Team C", Team2: "Team D", EpochTime: 2000},
	}
	b := []sources.ScheduledMatch{
		{Team1: "Team C", Team2: "Team D", EpochTime: 2000},
		{Team1: "Team A", Team2: "Team B", EpochTime: 1000},
	}
	assert.Equal(t, scheduleKey(a), scheduleKey(b))
}

func TestScheduleKey_DetectsTimeChange(t *testing.T) {
	before := []sources.ScheduledMatch{{Team1: "Team A", Team2: "Team B", EpochTime: 1000}}
	after := []sources.ScheduledMatch{{Team1: "Team A", Team2: "Team B", EpochTime: 9999}}
	assert.NotEqual(t, scheduleKey(before), scheduleKey(after))
}

func TestScheduleKey_DetectsTeamChange(t *testing.T) {
	before := []sources.ScheduledMatch{{Team1: "TBD", Team2: "Team B", EpochTime: 1000}}
	after := []sources.ScheduledMatch{{Team1: "Team A", Team2: "Team B", EpochTime: 1000}}
	assert.NotEqual(t, scheduleKey(before), scheduleKey(after))
}

func TestScheduleKey_EmptySlice(t *testing.T) {
	assert.Equal(t, scheduleKey(nil), scheduleKey([]sources.ScheduledMatch{}))
}

func TestScheduleKey_SameTeam1_SortsByTeam2(t *testing.T) {
	// Exercises the team2 comparison branch in the sort comparator.
	a := []sources.ScheduledMatch{
		{Team1: "Team A", Team2: "Zeta", EpochTime: 1000},
		{Team1: "Team A", Team2: "Alpha", EpochTime: 2000},
	}
	b := []sources.ScheduledMatch{
		{Team1: "Team A", Team2: "Alpha", EpochTime: 2000},
		{Team1: "Team A", Team2: "Zeta", EpochTime: 1000},
	}
	assert.Equal(t, scheduleKey(a), scheduleKey(b))
}

// endregion
