/* poller_test.go
 * Contains unit tests for poller.go functions
 * Authors: Zachary Bower
 */

package web

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// region NewPoller tests

func TestNewPoller_DefaultInterval(t *testing.T) {
	p := NewPoller(nil, 42, "test-key")

	assert.Equal(t, time.Minute, p.interval)
}

func TestNewPoller_Fields(t *testing.T) {
	p := NewPoller(nil, 99, "my-api-key")

	assert.Nil(t, p.app)
	assert.Equal(t, 99, p.seriesID)
	assert.Equal(t, "my-api-key", p.apiKey)
}

func TestNewPoller_KnownStatusInitialised(t *testing.T) {
	// knownStatus map must be initialised — a nil map panics on write
	p := NewPoller(nil, 1, "key")

	assert.NotNil(t, p.knownStatus)
	// writing to it should not panic
	p.knownStatus["test-id"] = "not_started"
}

// endregion

// region knownStatus transition logic tests

func TestPoller_StatusTransition_DetectsFinished(t *testing.T) {
	p := NewPoller(nil, 1, "key")
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
	p := NewPoller(nil, 1, "key")
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
	p := NewPoller(nil, 1, "key")

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
