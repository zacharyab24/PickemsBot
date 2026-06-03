/* liquipedia_test.go
 * Contains unit tests for liquipedia.go
 * Authors: Zachary Bower
 */

package sources

import (
	"bufio"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// region ParseLiquipediaMatches tests

func TestParseLiquipediaMatches(t *testing.T) {
	expectedResult := []MatchNode{
		{ID: "AmF15pUfHd_0001", Team1: "Aurora Gaming", Team2: "SAW", Winner: "TBD", Score: "", Section: "Round 4"},
		{ID: "AmF15pUfHd_0002", Team1: "Team Liquid", Team2: "FlyQuest", Winner: "TBD", Score: "", Section: "Round 4"},
		{ID: "AmF15pUfHd_0003", Team1: "B8", Team2: "Legacy", Winner: "TBD", Score: "", Section: "Round 4"},
		{ID: "IykJinz1G8_0001", Team1: "GamerLegion", Team2: "SAW", Winner: "SAW", Score: "1-2", Section: "Round 3"},
		{ID: "IykJinz1G8_0002", Team1: "Team Liquid", Team2: "BetBoom Team", Winner: "Team Liquid", Score: "2-1", Section: "Round 3"},
		{ID: "IykJinz1G8_0003", Team1: "3DMAX", Team2: "FlyQuest", Winner: "FlyQuest", Score: "1-2", Section: "Round 3"},
		{ID: "IykJinz1G8_0004", Team1: "Astralis", Team2: "Legacy", Winner: "Legacy", Score: "0-2", Section: "Round 3"},
		{ID: "U7JeCe3nrs_0001", Team1: "GamerLegion", Team2: "PaiN Gaming", Winner: "PaiN Gaming", Score: "1-2", Section: "Round 2"},
		{ID: "U7JeCe3nrs_0002", Team1: "HEROIC", Team2: "BetBoom Team", Winner: "HEROIC", Score: "2-0", Section: "Round 2"},
		{ID: "U7JeCe3nrs_0003", Team1: "Aurora Gaming", Team2: "Team Liquid", Winner: "Aurora Gaming", Score: "2-1", Section: "Round 2"},
		{ID: "U7JeCe3nrs_0004", Team1: "3DMAX", Team2: "B8", Winner: "B8", Score: "0-2", Section: "Round 2"},
		{ID: "VKTHpS7s0x_0001", Team1: "Aurora Gaming", Team2: "HEROIC", Winner: "HEROIC", Score: "0-2", Section: "Round 3"},
		{ID: "VKTHpS7s0x_0002", Team1: "B8", Team2: "PaiN Gaming", Winner: "PaiN Gaming", Score: "0-2", Section: "Round 3"},
		{ID: "ayB546T4zZ_0001", Team1: "PaiN Gaming", Team2: "Gentle Mates", Winner: "PaiN Gaming", Score: "2-0", Section: "Round 1"},
		{ID: "ayB546T4zZ_0002", Team1: "Legacy", Team2: "Team Liquid", Winner: "Team Liquid", Score: "1-2", Section: "Round 1"},
		{ID: "ayB546T4zZ_0003", Team1: "HEROIC", Team2: "Ninjas in Pyjamas", Winner: "HEROIC", Score: "2-0", Section: "Round 1"},
		{ID: "ayB546T4zZ_0004", Team1: "GamerLegion", Team2: "FlyQuest", Winner: "GamerLegion", Score: "2-0", Section: "Round 1"},
		{ID: "ayB546T4zZ_0005", Team1: "3DMAX", Team2: "SAW", Winner: "3DMAX", Score: "2-0", Section: "Round 1"},
		{ID: "ayB546T4zZ_0006", Team1: "BetBoom Team", Team2: "MIBR", Winner: "BetBoom Team", Score: "2-1", Section: "Round 1"},
		{ID: "ayB546T4zZ_0007", Team1: "Aurora Gaming", Team2: "Fnatic", Winner: "Aurora Gaming", Score: "2-0", Section: "Round 1"},
		{ID: "ayB546T4zZ_0008", Team1: "Astralis", Team2: "B8", Winner: "B8", Score: "0-2", Section: "Round 1"},
		{ID: "f3Ubb66fCx_0001", Team1: "3DMAX", Team2: "Astralis", Winner: "TBD", Score: "", Section: "Round 4"},
		{ID: "f3Ubb66fCx_0002", Team1: "BetBoom Team", Team2: "Gentle Mates", Winner: "TBD", Score: "", Section: "Round 4"},
		{ID: "f3Ubb66fCx_0003", Team1: "GamerLegion", Team2: "Fnatic", Winner: "TBD", Score: "", Section: "Round 4"},
		{ID: "ilPVE8BYF6_0001", Team1: "Ninjas in Pyjamas", Team2: "Gentle Mates", Winner: "Gentle Mates", Score: "1-2", Section: "Round 3"},
		{ID: "ilPVE8BYF6_0002", Team1: "Fnatic", Team2: "MIBR", Winner: "Fnatic", Score: "2-0", Section: "Round 3"},
		{ID: "vINHUV3all_0001", Team1: "Legacy", Team2: "Gentle Mates", Winner: "Legacy", Score: "2-0", Section: "Round 2"},
		{ID: "vINHUV3all_0002", Team1: "SAW", Team2: "Ninjas in Pyjamas", Winner: "SAW", Score: "2-1", Section: "Round 2"},
		{ID: "vINHUV3all_0003", Team1: "Fnatic", Team2: "FlyQuest", Winner: "FlyQuest", Score: "0-2", Section: "Round 2"},
		{ID: "vINHUV3all_0004", Team1: "Astralis", Team2: "MIBR", Winner: "Astralis", Score: "2-0", Section: "Round 2"},
		{ID: "zIiQwLgw83_0001", Team1: "TBD", Team2: "TBD", Winner: "TBD", Score: "", Section: "Round 5"},
		{ID: "zIiQwLgw83_0002", Team1: "TBD", Team2: "TBD", Winner: "TBD", Score: "", Section: "Round 5"},
		{ID: "zIiQwLgw83_0003", Team1: "TBD", Team2: "TBD", Winner: "TBD", Score: "", Section: "Round 5"},
	}

	f, err := os.Open("testdata/parser/scheduledMatchRawData.json")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	var sb strings.Builder
	for scanner.Scan() {
		sb.WriteString(scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}

	actualResult, err := ParseLiquipediaMatches(sb.String())
	if err != nil {
		t.Fatal("ParseLiquipediaMatches should not have thrown an error. Error:", err)
	}
	if !reflect.DeepEqual(actualResult, expectedResult) {
		t.Fatal("Actual does not equal expected")
	}
}

func TestParseLiquipediaMatches_NoMatchNodes(t *testing.T) {
	matches, err := ParseLiquipediaMatches("{\"result\" : []}")
	if err != nil {
		t.Fatal("ParseLiquipediaMatches should not have returned an error")
	}
	if matches != nil {
		t.Fatal("expected matches to be nil")
	}
}

func TestParseLiquipediaMatches_InvalidJson(t *testing.T) {
	_, err := ParseLiquipediaMatches("{\"some invalid json\"}")
	if err == nil {
		t.Fatal("ParseLiquipediaMatches should have returned an error")
	}
	if err.Error() != "error parsing JSON: invalid character '}' after object key" {
		t.Fatal("Unexpected error message")
	}
}

func TestParseLiquipediaMatches_NoResultField(t *testing.T) {
	_, err := ParseLiquipediaMatches("{\"key\" : \"value\"}")
	if err == nil {
		t.Fatal("ParseLiquipediaMatches should have returned an error")
	}
	if err.Error() != "missing or invalid 'result' field" {
		t.Fatal("Unexpected error message")
	}
}

func TestParseLiquipediaMatches_InvalidData(t *testing.T) {
	f, err := os.Open("testdata/parser/scheduledMatchInvalidRawData.json")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	var sb strings.Builder
	for scanner.Scan() {
		sb.WriteString(scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}

	_, err = ParseLiquipediaMatches(sb.String())
	if err == nil {
		t.Fatal("ParseLiquipediaMatches should have returned an error")
	}
	if strings.Contains(err.Error(), "Error creating match node") {
		t.Fatal("Unexpected error message")
	}
}

// endregion

// region ParseLiquipediaSchedule tests

func TestParseLiquipediaSchedule(t *testing.T) {
	expectedResult := []ScheduledMatch{
		{"TBD", "TBD", -62167219200, "3", "PGL", false, false},
		{"TBD", "TBD", -62167219200, "3", "PGL", false, false},
		{"TBD", "TBD", -62167219200, "3", "PGL", false, false},
		{"Legacy", "Team Liquid", 1761465600, "3", "PGL", true, false},
		{"PaiN Gaming", "Gentle Mates", 1761465900, "3", "PGL_CS2", true, false},
		{"HEROIC", "Ninjas in Pyjamas", 1761476400, "3", "PGL_CS2", true, false},
		{"GamerLegion", "FlyQuest", 1761477300, "3", "PGL", true, false},
		{"BetBoom Team", "MIBR", 1761483300, "3", "PGL_CS2", true, false},
		{"3DMAX", "SAW", 1761488400, "3", "PGL", true, false},
		{"Astralis", "B8", 1761494700, "3", "PGL_CS2", true, false},
		{"Aurora Gaming", "Fnatic", 1761495600, "3", "PGL", true, false},
		{"GamerLegion", "PaiN Gaming", 1761552000, "3", "PGL", true, false},
		{"Legacy", "Gentle Mates", 1761552000, "3", "PGL_CS2", true, false},
		{"SAW", "Ninjas in Pyjamas", 1761560100, "3", "PGL_CS2", true, false},
		{"HEROIC", "BetBoom Team", 1761564000, "3", "PGL", true, false},
		{"Fnatic", "FlyQuest", 1761571200, "3", "PGL_CS2", true, false},
		{"Aurora Gaming", "Team Liquid", 1761572700, "3", "PGL", true, false},
		{"Astralis", "MIBR", 1761579900, "3", "PGL_CS2", true, false},
		{"3DMAX", "B8", 1761584700, "3", "PGL", true, false},
		{"Team Liquid", "BetBoom Team", 1761638400, "3", "PGL", true, false},
		{"GamerLegion", "SAW", 1761638400, "3", "PGL_CS2", true, false},
		{"Astralis", "Legacy", 1761647700, "3", "PGL", true, false},
		{"3DMAX", "FlyQuest", 1761653100, "3", "PGL_CS2", true, false},
		{"Aurora Gaming", "HEROIC", 1761656100, "3", "PGL", true, false},
		{"B8", "PaiN Gaming", 1761663600, "3", "PGL", true, false},
		{"Ninjas in Pyjamas", "Gentle Mates", 1761664200, "3", "PGL_CS2", true, false},
		{"Fnatic", "MIBR", 1761675900, "3", "PGL_CS2", true, false},
		{"Aurora Gaming", "SAW", 1761724800, "3", "PGL", false, false},
		{"3DMAX", "Astralis", 1761724800, "3", "PGL_CS2", false, false},
		{"BetBoom Team", "Gentle Mates", 1761735600, "3", "PGL_CS2", false, false},
		{"Team Liquid", "FlyQuest", 1761735600, "3", "PGL", false, false},
		{"GamerLegion", "Fnatic", 1761746400, "3", "PGL_CS2", false, false},
		{"B8", "Legacy", 1761746400, "3", "PGL", false, false},
	}
	sort.Slice(expectedResult, func(i, j int) bool {
		return expectedResult[i].EpochTime < expectedResult[j].EpochTime
	})

	f, err := os.Open("testdata/parser/scheduledMatchRawData.json")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	var sb strings.Builder
	for scanner.Scan() {
		sb.WriteString(scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}

	actualResult, err := ParseLiquipediaSchedule(sb.String())
	sort.Slice(actualResult, func(i, j int) bool {
		return actualResult[i].EpochTime < actualResult[j].EpochTime
	})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(actualResult, expectedResult) {
		t.Fatal("Actual does not equal expected")
	}
}

func TestParseLiquipediaSchedule_NoMatchNodes(t *testing.T) {
	matches, err := ParseLiquipediaSchedule("{\"result\" : []}")
	if err != nil {
		t.Fatal("ParseLiquipediaMatches should not have returned an error")
	}
	if matches != nil {
		t.Fatal("expected matches to be nil")
	}
}

func TestParseLiquipediaSchedule_InvalidJson(t *testing.T) {
	_, err := ParseLiquipediaSchedule("{\"some invalid json\"}")
	if err == nil {
		t.Fatal("ParseLiquipediaMatches should have returned an error")
	}
	if err.Error() != "error parsing JSON: invalid character '}' after object key" {
		t.Fatal("Unexpected error message")
	}
}

func TestParseLiquipediaSchedule_NoResultField(t *testing.T) {
	_, err := ParseLiquipediaSchedule("{\"key\" : \"value\"}")
	if err == nil {
		t.Fatal("ParseLiquipediaMatches should have returned an error")
	}
	if err.Error() != "missing or invalid 'result' field" {
		t.Fatal("Unexpected error message")
	}
}

func TestParseLiquipediaSchedule_InvalidData(t *testing.T) {
	f, err := os.Open("testdata/parser/scheduledMatchInvalidRawData.json")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	var sb strings.Builder
	for scanner.Scan() {
		sb.WriteString(scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}

	_, err = ParseLiquipediaSchedule(sb.String())
	if err == nil {
		t.Fatal("ParseLiquipediaMatches should have returned an error")
	}
	if strings.Contains(err.Error(), "Error creating match node") {
		t.Fatal("Unexpected error message")
	}
}

// endregion

// region parseLiquipediaScheduledMatch tests

func TestParseScheduledMatches_Success(t *testing.T) {
	matchData := map[string]interface{}{
		"finished": float64(0),
		"date":     "2025-01-15 14:00:00",
		"stream": map[string]interface{}{
			"twitch": "esl_csgo",
		},
		"bestof": float64(3),
		"match2opponents": []interface{}{
			map[string]interface{}{"name": "Team A"},
			map[string]interface{}{"name": "Team B"},
		},
	}

	result, err := parseLiquipediaScheduledMatch(matchData)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if result.Team1 != "Team A" {
		t.Errorf("Expected Team A, got %s", result.Team1)
	}
	if result.Team2 != "Team B" {
		t.Errorf("Expected Team B, got %s", result.Team2)
	}
	if result.BestOf != "3" {
		t.Errorf("Expected 3, got %s", result.BestOf)
	}
	if result.Finished {
		t.Error("Expected Finished to be false")
	}
}

func TestParseScheduledMatches_FinishedMatch(t *testing.T) {
	matchData := map[string]interface{}{
		"finished": float64(1),
		"date":     "2025-01-15 14:00:00",
		"stream": map[string]interface{}{
			"twitch": "esl_csgo",
		},
		"bestof": float64(3),
		"match2opponents": []interface{}{
			map[string]interface{}{"name": "Team A"},
			map[string]interface{}{"name": "Team B"},
		},
	}

	result, err := parseLiquipediaScheduledMatch(matchData)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if !result.Finished {
		t.Error("Expected Finished to be true")
	}
}

func TestParseScheduledMatches_KickStream(t *testing.T) {
	matchData := map[string]interface{}{
		"finished": float64(0),
		"date":     "2025-01-15 14:00:00",
		"stream": map[string]interface{}{
			"kick": "blast_tv",
		},
		"bestof": float64(3),
		"match2opponents": []interface{}{
			map[string]interface{}{"name": "Team A"},
			map[string]interface{}{"name": "Team B"},
		},
	}

	result, err := parseLiquipediaScheduledMatch(matchData)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if result.StreamURL != "blast_tv" {
		t.Errorf("Expected blast_tv, got %s", result.StreamURL)
	}
}

func TestParseScheduledMatches_TBDTeam(t *testing.T) {
	matchData := map[string]interface{}{
		"finished": float64(0),
		"date":     "2025-01-15 14:00:00",
		"stream": map[string]interface{}{
			"twitch": "esl_csgo",
		},
		"bestof": float64(3),
		"match2opponents": []interface{}{
			map[string]interface{}{"name": ""},
			map[string]interface{}{"name": "Team B"},
		},
	}

	result, err := parseLiquipediaScheduledMatch(matchData)

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if result.Team1 != "TBD" {
		t.Errorf("Expected TBD, got %s", result.Team1)
	}
}

func TestParseScheduledMatches_InvalidType(t *testing.T) {
	_, err := parseLiquipediaScheduledMatch("not a map")

	if err == nil {
		t.Fatal("Expected error for invalid type")
	}
}

func TestParseScheduledMatches_MissingFinished(t *testing.T) {
	matchData := map[string]interface{}{
		"date": "2025-01-15 14:00:00",
	}

	_, err := parseLiquipediaScheduledMatch(matchData)

	if err == nil {
		t.Fatal("Expected error for missing finished field")
	}
}

func TestParseScheduledMatches_InvalidFinishedValue(t *testing.T) {
	matchData := map[string]interface{}{
		"finished": float64(2),
		"date":     "2025-01-15 14:00:00",
	}

	_, err := parseLiquipediaScheduledMatch(matchData)

	if err == nil {
		t.Fatal("Expected error for invalid finished value")
	}
}

func TestParseScheduledMatches_NoStreamKeys(t *testing.T) {
	// Matches with no twitch/kick stream (e.g. YouTube-only or unassigned) should
	// parse successfully with an empty StreamURL rather than returning an error.
	matchData := map[string]interface{}{
		"finished": float64(0),
		"date":     "2025-01-15 14:00:00",
		"stream": map[string]interface{}{
			"youtube": "some_channel",
		},
		"bestof": float64(3),
		"match2opponents": []interface{}{
			map[string]interface{}{"name": "Team A"},
			map[string]interface{}{"name": "Team B"},
		},
	}

	match, err := parseLiquipediaScheduledMatch(matchData)

	if err != nil {
		t.Fatalf("Expected no error for missing twitch/kick keys, got: %v", err)
	}
	if match.StreamURL != "" {
		t.Errorf("Expected empty StreamURL for unrecognised stream platform, got: %q", match.StreamURL)
	}
}

// endregion
