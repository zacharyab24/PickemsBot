/* matches.go
 * Contains the interfaces, structs and helper functions used by the match package related to data fetching
 * Authors: Zachary Bower
 * Last modified: 28/05/2025
 */

package match

import (
	"fmt"
	"strings"
)

// Interface for MatchResults. Used to unify the return types of swiss and single-elimination for GetMatchData
type MatchResult interface {
	GetType() string
}

// Struct for swiss results
type SwissResult struct {
	Scores map[string]string
}

func (s SwissResult) GetType() string {
	return "swiss"
}

// Helper struct to track if the team has advanced past a round of the elim bracket, been eliminated or pending
type TeamProgress struct {
	Round string
	Status string
}

// Struct for single elimination results
type EliminationResult struct {
    Progression map[string]TeamProgress 
}

func (e EliminationResult) GetType() string {
    return "single-elimination"
}

// Struct for a binary tree node
// This tree is used for the results of the finals section, or any other single elimination tournament
type MatchNode struct {
	Id string
	Team1 string
	Team2 string
	Winner string
	Left *MatchNode
	Right *MatchNode
}

type UpcomingMatch struct {
	Team1 string
	Team2 string
	EpochTime int64
	BestOf string
	StreamUrl string
}

// Function to print teh EliminationResult tree by level (breadth-first)
func PrintTreeLevelOrder(root *MatchNode) {
    if root == nil {
        fmt.Println("Empty tree")
        return
    }
    
    fmt.Println("Tournament Tree (Level Order):")
    fmt.Println(strings.Repeat("=", 60))
    
    queue := []*MatchNode{root}
    level := 0
    
    for len(queue) > 0 {
        levelSize := len(queue)
        fmt.Printf("Level %d:\n", level+1)
        
        for i := 0; i < levelSize; i++ {
            node := queue[0]
            queue = queue[1:]
            
            winner := node.Winner
            if winner == "" {
                winner = "TBD"
            }
            
            fmt.Printf("  %s vs %s (Winner: %s)\n", node.Team1, node.Team2, winner)
            
            if node.Left != nil {
                queue = append(queue, node.Left)
            }
            if node.Right != nil {
                queue = append(queue, node.Right)
            }
        }
        fmt.Println()
        level++
    }
}