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

// region GetMatchNodesFromJson tests

// Test of GetMatchNodesFromJson that tests normal flow returns expected result
func TestGetMatchNodesFromJson(t *testing.T) {
	// Seed data
	expectedResult := []MatchNode{
		{"AmF15pUfHd_0001", "Aurora Gaming", "SAW", "TBD"},
		{"AmF15pUfHd_0002", "Team Liquid", "FlyQuest", "TBD"},
		{"AmF15pUfHd_0003", "B8", "Legacy", "TBD"},
		{"IykJinz1G8_0001", "GamerLegion", "SAW", "SAW"},
		{"IykJinz1G8_0002", "Team Liquid", "BetBoom Team", "Team Liquid"},
		{"IykJinz1G8_0003", "3DMAX", "FlyQuest", "FlyQuest"},
		{"IykJinz1G8_0004", "Astralis", "Legacy", "Legacy"},
		{"U7JeCe3nrs_0001", "GamerLegion", "PaiN Gaming", "PaiN Gaming"},
		{"U7JeCe3nrs_0002", "HEROIC", "BetBoom Team", "HEROIC"},
		{"U7JeCe3nrs_0003", "Aurora Gaming", "Team Liquid", "Aurora Gaming"},
		{"U7JeCe3nrs_0004", "3DMAX", "B8", "B8"},
		{"VKTHpS7s0x_0001", "Aurora Gaming", "HEROIC", "HEROIC"},
		{"VKTHpS7s0x_0002", "B8", "PaiN Gaming", "PaiN Gaming"},
		{"ayB546T4zZ_0001", "PaiN Gaming", "Gentle Mates", "PaiN Gaming"},
		{"ayB546T4zZ_0002", "Legacy", "Team Liquid", "Team Liquid"},
		{"ayB546T4zZ_0003", "HEROIC", "Ninjas in Pyjamas", "HEROIC"},
		{"ayB546T4zZ_0004", "GamerLegion", "FlyQuest", "GamerLegion"},
		{"ayB546T4zZ_0005", "3DMAX", "SAW", "3DMAX"},
		{"ayB546T4zZ_0006", "BetBoom Team", "MIBR", "BetBoom Team"},
		{"ayB546T4zZ_0007", "Aurora Gaming", "Fnatic", "Aurora Gaming"},
		{"ayB546T4zZ_0008", "Astralis", "B8", "B8"},
		{"f3Ubb66fCx_0001", "3DMAX", "Astralis", "TBD"},
		{"f3Ubb66fCx_0002", "BetBoom Team", "Gentle Mates", "TBD"},
		{"f3Ubb66fCx_0003", "GamerLegion", "Fnatic", "TBD"},
		{"ilPVE8BYF6_0001", "Ninjas in Pyjamas", "Gentle Mates", "Gentle Mates"},
		{"ilPVE8BYF6_0002", "Fnatic", "MIBR", "Fnatic"},
		{"vINHUV3all_0001", "Legacy", "Gentle Mates", "Legacy"},
		{"vINHUV3all_0002", "SAW", "Ninjas in Pyjamas", "SAW"},
		{"vINHUV3all_0003", "Fnatic", "FlyQuest", "FlyQuest"},
		{"vINHUV3all_0004", "Astralis", "MIBR", "Astralis"},
		{"zIiQwLgw83_0001", "TBD", "TBD", "TBD"},
		{"zIiQwLgw83_0002", "TBD", "TBD", "TBD"},
		{"zIiQwLgw83_0003", "TBD", "TBD", "TBD"},
	}

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
	actualResult, err := GetMatchNodesFromJson(rawJson)
	if err != nil {
		t.Fatal("GetMatchNodesFromJson should not have thrown an error. Error:", err)
	}
	if !reflect.DeepEqual(actualResult, expectedResult) {
		t.Fatal("Actual does not equal expected")
	}
}

// Test of GetMatchNodesFromJson when no data is returned
func TestGetMatchNodesFromJson_NoMatchNodes(t *testing.T) {
	inputString := "{\"result\" : []}"
	matches, err := GetMatchNodesFromJson(inputString)
	if err != nil {
		t.Fatal("GetMatchNodesFromJson should not have returned an error")
	}
	if matches != nil {
		t.Fatal("expected matches to be nil")
	}
}

// Test of GetMatchNodesFromJson when given invalid json
func TestGetMatchNodesFromJson_InvalidJson(t *testing.T) {
	inputString := "{\"some invalid json\"}"
	_, err := GetMatchNodesFromJson(inputString)
	if err == nil {
		t.Fatal("GetMatchNodesFromJson should have returned an error")
	}
	if err.Error() != "error parsing JSON: invalid character '}' after object key" {
		t.Fatal("Unexpected error message")
	}
}

// Test of GetMatchNodesFromJson when given valid json with invalid data
func TestGetMatchNodesFromJson_NoResultField(t *testing.T) {
	inputString := "{\"key\" : \"value\"}"
	_, err := GetMatchNodesFromJson(inputString)
	if err == nil {
		t.Fatal("GetMatchNodesFromJson should have returned an error")
	}
	if err.Error() != "missing or invalid 'result' field" {
		t.Fatal("Unexpected error message")
	}
}

// Test of GetMatchNodesFromJson when ParseScheduledMatches throws an error
func TestGetMatchNodesFromJson_InvalidData(t *testing.T) {
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
	_, err = GetMatchNodesFromJson(rawJson)
	if err == nil {
		t.Fatal("GetMatchNodesFromJson should have returned an error")
	}
	if strings.Contains(err.Error(), "Error creating match node") {
		t.Fatal("Unexpected error message")
	}
}

// endregion

// region GetScheduledMatchesFromJson tests

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

//endregion

// TODO: Add comprehensive ParseMatchData tests with proper map[string]interface{} input
