/* poller_test.go
 * Contains unit tests for poller.go functions
 * Authors: Zachary Bower
 */

package web

import (
	"log/slog"
	"pickems-bot/sources"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// region NewPoller tests

func TestNewPoller_DefaultInterval(t *testing.T) {
	p := NewPoller(nil, 42, 0, 0, "", "test-key", "", nil)

	assert.Equal(t, time.Minute, p.interval)
}

func TestNewPoller_Fields(t *testing.T) {
	p := NewPoller(nil, 99, 0, 0, "", "my-api-key", "", nil)

	assert.Nil(t, p.app)
	assert.Equal(t, 99, p.seriesID)
	assert.Equal(t, "my-api-key", p.apiKey)
}

func TestNewPoller_KnownStatusInitialised(t *testing.T) {
	// knownStatus map must be initialised — a nil map panics on write
	p := NewPoller(nil, 1, 0, 0, "", "key", "", nil)

	assert.NotNil(t, p.knownStatus)
	// writing to it should not panic
	p.knownStatus["test-id"] = "not_started"
}

// endregion

// region logger tests

func TestPoller_Logger_NilLog_ReturnsDefault(t *testing.T) {
	p := NewPoller(nil, 1, 0, 0, "", "key", "", nil)
	l := p.logger()
	assert.NotNil(t, l)
}

func TestPoller_Logger_InjectedLog(t *testing.T) {
	p := NewPoller(nil, 1, 0, 0, "", "key", "", slog.Default())
	l := p.logger()
	assert.NotNil(t, l)
}

// endregion

// region knownStatus transition logic tests

func TestPoller_StatusTransition_DetectsFinished(t *testing.T) {
	p := NewPoller(nil, 1, 0, 0, "", "key", "", nil)
	p.knownStatus["match-1"] = "running"

	// Simulate a tick where match-1 transitions to finished
	finishedTransition := false
	statuses := map[string]string{"match-1": "finished"}

	for id, status := range statuses {
		prev, seen := p.knownStatus[id]
		if seen && prev != "finished" && status == "finished" {
			finishedTransition = true
		}
		p.knownStatus[id] = status
	}

	assert.True(t, finishedTransition)
	assert.Equal(t, "finished", p.knownStatus["match-1"])
}

func TestPoller_StatusTransition_NoTriggerIfAlreadyFinished(t *testing.T) {
	// A match already marked finished should not trigger again on next tick
	p := NewPoller(nil, 1, 0, 0, "", "key", "", nil)
	p.knownStatus["match-1"] = "finished"

	finishedTransition := false
	statuses := map[string]string{"match-1": "finished"}

	for id, status := range statuses {
		prev, seen := p.knownStatus[id]
		if seen && prev != "finished" && status == "finished" {
			finishedTransition = true
		}
		p.knownStatus[id] = status
	}

	assert.False(t, finishedTransition)
}

func TestPoller_StatusTransition_NoTriggerForFirstSeen(t *testing.T) {
	// A brand-new match seen as "finished" (never tracked before) should not trigger —
	// we only react to transitions, not initial state.
	p := NewPoller(nil, 1, 0, 0, "", "key", "", nil)

	finishedTransition := false
	statuses := map[string]string{"match-new": "finished"}

	for id, status := range statuses {
		prev, seen := p.knownStatus[id]
		if seen && prev != "finished" && status == "finished" {
			finishedTransition = true
		}
		p.knownStatus[id] = status
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
