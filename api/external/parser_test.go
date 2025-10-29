/* parser_test.go
 * Contains unit tests for parser.go
 * Authors: Zachary Bower
 */

package external

import (
	"fmt"
	"os"
	"testing"
)

func TestGetMatchNodesFromJson(t *testing.T) {

	apiKey := os.Getenv("LIQUIDPEDIADB_API_KEY")
	scheduledMatches, err := FetchScheduledMatches(apiKey, "PGL/2025", "")
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(scheduledMatches)

	//os.WriteFile("C:/Users/zacbo/Downloads/Pickems_test/scheduledMatches.json")
}
