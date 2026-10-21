package app

import (
	"net/http"
	"testing"

	"pickems-bot/internal/testutil"
	"pickems-bot/store"
	"pickems-bot/tournament"
)

func TestCheckAndStoreFormat_PersistsDetectedKind(t *testing.T) {
	var gotPath string
	testutil.RouteDefaultClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write([]byte(`[
			{"id": 1, "previous_matches": []},
			{"id": 2, "previous_matches": []},
			{"id": 3, "previous_matches": [{"match_id": 1, "type": "winner"}, {"match_id": 2, "type": "winner"}]}
		]`))
	})
	ms := NewMockStore("", "Playoffs")
	api := NewTestApp(ms)

	err := api.checkAndStoreFormat(bg(), store.Tournament{ID: 9, Source: "pandascore", ExternalID: "123"})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if gotPath != "/tournaments/123/brackets" {
		t.Errorf("fetched %s, want the bracket for external id 123", gotPath)
	}
	if ms.SetFormatID != 9 || ms.SetFormatValue != string(tournament.SingleElim) {
		t.Errorf("stored format (%d, %q), want (9, %q)", ms.SetFormatID, ms.SetFormatValue, tournament.SingleElim)
	}
}

// An empty bracket is ambiguous; the format stays NULL so match-node detection
// can classify it later.
func TestCheckAndStoreFormat_EmptyBracket_LeavesFormatUnset(t *testing.T) {
	testutil.RouteDefaultClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[]`))
	})
	ms := NewMockStore("", "Playoffs")

	if err := NewTestApp(ms).checkAndStoreFormat(bg(), store.Tournament{ID: 9, Source: "pandascore", ExternalID: "123"}); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if ms.SetFormatID != 0 {
		t.Errorf("expected no format to be stored, got %q", ms.SetFormatValue)
	}
}

func TestCheckAndStoreFormat_SkipsWithoutFetching(t *testing.T) {
	format := string(tournament.Swiss)
	tests := map[string]store.Tournament{
		"liquipedia tournament": {ID: 9, Source: "liquipedia", ExternalID: "Some/Page"},
		"format already known":  {ID: 9, Source: "pandascore", ExternalID: "123", Format: &format},
	}
	for name, tourney := range tests {
		t.Run(name, func(t *testing.T) {
			testutil.RouteDefaultClient(t, func(w http.ResponseWriter, r *http.Request) {
				t.Errorf("unexpected request to %s", r.URL.Path)
			})
			ms := NewMockStore("", "Playoffs")

			if err := NewTestApp(ms).checkAndStoreFormat(bg(), tourney); err != nil {
				t.Fatalf("expected no error, got: %v", err)
			}
			if ms.SetFormatID != 0 {
				t.Error("expected no format to be stored")
			}
		})
	}
}

func TestCheckAndStoreFormat_Errors(t *testing.T) {
	tests := map[string]struct {
		externalID string
		status     int
	}{
		"non-numeric external id": {"abc", http.StatusOK},
		"bracket not found":       {"123", http.StatusNotFound},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			testutil.RouteDefaultClient(t, func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
			})
			ms := NewMockStore("", "Playoffs")

			err := NewTestApp(ms).checkAndStoreFormat(bg(), store.Tournament{ID: 9, Source: "pandascore", ExternalID: tt.externalID})
			if err == nil {
				t.Fatal("expected an error")
			}
			if ms.SetFormatID != 0 {
				t.Error("expected no format to be stored")
			}
		})
	}
}
