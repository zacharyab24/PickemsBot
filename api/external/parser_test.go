/* parser_test.go
 * Contains unit tests for parser.go
 * Authors: Zachary Bower
 */

package external

import (
	"bufio"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// Test of GetScheduledMatchesFromJson that tests normal flow returns expected result
func TestGetScheduledMatchesFromJson(t *testing.T) {
	// Seed data
	expectedResult := []ScheduledMatch{
		{"TBD", "TBD", -62167219200, "3", "PGL", false},
		{"TBD", "TBD", -62167219200, "3", "PGL", false},
		{"TBD", "TBD", -62167219200, "3", "PGL", false},
		{"Legacy", "Team Liquid", 1761465600, "3", "PGL", true},
		{"PaiN Gaming", "Gentle Mates", 1761465900, "3", "PGL_CS2", true},
		{"HEROIC", "Ninjas in Pyjamas", 1761476400, "3", "PGL_CS2", true},
		{"GamerLegion", "FlyQuest", 1761477300, "3", "PGL", true},
		{"BetBoom Team", "MIBR", 1761483300, "3", "PGL_CS2", true},
		{"3DMAX", "SAW", 1761488400, "3", "PGL", true},
		{"Astralis", "B8", 1761494700, "3", "PGL_CS2", true},
		{"Aurora Gaming", "Fnatic", 1761495600, "3", "PGL", true},
		{"GamerLegion", "PaiN Gaming", 1761552000, "3", "PGL", true},
		{"Legacy", "Gentle Mates", 1761552000, "3", "PGL_CS2", true},
		{"SAW", "Ninjas in Pyjamas", 1761560100, "3", "PGL_CS2", true},
		{"HEROIC", "BetBoom Team", 1761564000, "3", "PGL", true},
		{"Fnatic", "FlyQuest", 1761571200, "3", "PGL_CS2", true},
		{"Aurora Gaming", "Team Liquid", 1761572700, "3", "PGL", true},
		{"Astralis", "MIBR", 1761579900, "3", "PGL_CS2", true},
		{"3DMAX", "B8", 1761584700, "3", "PGL", true},
		{"Team Liquid", "BetBoom Team", 1761638400, "3", "PGL", true},
		{"GamerLegion", "SAW", 1761638400, "3", "PGL_CS2", true},
		{"Astralis", "Legacy", 1761647700, "3", "PGL", true},
		{"3DMAX", "FlyQuest", 1761653100, "3", "PGL_CS2", true},
		{"Aurora Gaming", "HEROIC", 1761656100, "3", "PGL", true},
		{"B8", "PaiN Gaming", 1761663600, "3", "PGL", true},
		{"Ninjas in Pyjamas", "Gentle Mates", 1761664200, "3", "PGL_CS2", true},
		{"Fnatic", "MIBR", 1761675900, "3", "PGL_CS2", true},
		{"Aurora Gaming", "SAW", 1761724800, "3", "PGL", false},
		{"3DMAX", "Astralis", 1761724800, "3", "PGL_CS2", false},
		{"BetBoom Team", "Gentle Mates", 1761735600, "3", "PGL_CS2", false},
		{"Team Liquid", "FlyQuest", 1761735600, "3", "PGL", false},
		{"GamerLegion", "Fnatic", 1761746400, "3", "PGL_CS2", false},
		{"B8", "Legacy", 1761746400, "3", "PGL", false},
	}
	sort.Slice(expectedResult, func(i, j int) bool {
		return expectedResult[i].EpochTime < expectedResult[j].EpochTime
	})

	// Load json from disk
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

	rawJson := sb.String()
	actualResult, err := GetScheduledMatchesFromJson(rawJson)
	sort.Slice(actualResult, func(i, j int) bool {
		return actualResult[i].EpochTime < actualResult[j].EpochTime
	})
	if err != nil {
		t.Fatal(err)
	}

	// Test results are matching

	if !reflect.DeepEqual(actualResult, expectedResult) {
		t.Fatal("Actual does not equal expected")
	}
}

// Test of GetScheduledMatchesFromJson when no data is returned
func TestGetScheduledMatchesFromJson_NoMatchNodes(t *testing.T) {
	inputString := "{\"result\" : []}"
	matches, err := GetScheduledMatchesFromJson(inputString)
	if err != nil {
		t.Fatal("GetMatchNodesFromJson should not have returned an error")
	}
	if matches != nil {
		t.Fatal("expected matches to be nil")
	}
}

// Test of GetScheduledMatchesFromJson when given invalid json
func TestGetScheduledMatchesFromJson_InvalidJson(t *testing.T) {
	inputString := "{\"some invalid json\"}"
	_, err := GetScheduledMatchesFromJson(inputString)
	if err == nil {
		t.Fatal("GetMatchNodesFromJson should have returned an error")
	}
	if err.Error() != "error parsing JSON: invalid character '}' after object key" {
		t.Fatal("Unexpected error message")
	}
}

// Test of GetScheduledMatchesFromJson when given valid json with invalid data
func TestGetScheduledMatchesFromJson_NoResultField(t *testing.T) {
	inputString := "{\"key\" : \"value\"}"
	_, err := GetScheduledMatchesFromJson(inputString)
	if err == nil {
		t.Fatal("GetMatchNodesFromJson should have returned an error")
	}
	if err.Error() != "missing or invalid 'result' field" {
		t.Fatal("Unexpected error message")
	}
}

// Test of GetScheduledMatchesFromJson when ParseScheduledMatches throws an error
func TestGetScheduledMatchesFromJson_InvalidData(t *testing.T) {
	// Load json from disk
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

	rawJson := sb.String()
	_, err = GetScheduledMatchesFromJson(rawJson)
	if err == nil {
		t.Fatal("GetMatchNodesFromJson should have returned an error")
	}
	if strings.Contains(err.Error(), "Error creating match node") {
		t.Fatal("Unexpected error message")
	}
}
