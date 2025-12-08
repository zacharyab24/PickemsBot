/* utils.go
 * Utility functions used across the application
 * Authors: Zachary Bower
 */

package main

import (
	"fmt"
	"strings"
)

// convertStrToBool converts a string of true or false into a boolean for comparisons
// Preconditions: Receives string containing either true or false (case insensitive)
// Postconditions: Returns boolean value or an error if the string is not true or false
func convertStrToBool(str string) (bool, error) {
	str = strings.TrimSpace(str)
	str = strings.ToLower(str)

	if str == "true" {
		return true, nil
	} else if str == "false" {
		return false, nil
	}
	return false, fmt.Errorf("invalid boolean string")
}
