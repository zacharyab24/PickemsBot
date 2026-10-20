package bot

import (
	"strings"
	"testing"

	"pickems-bot/app"
)

func leaderboardUsers(n int) []app.LeaderboardUser {
	users := make([]app.LeaderboardUser, n)
	for i := range users {
		users[i] = app.LeaderboardUser{UserID: string(rune('a' + i)), Username: string(rune('A' + i)), Rank: i + 1}
	}
	return users
}

func TestLeaderboardText_AllFit(t *testing.T) {
	got := leaderboardText(leaderboardUsers(3), "", maxMessageText)
	want := "1. **A** - 0 correct, 0 incorrect\n2. **B** - 0 correct, 0 incorrect\n3. **C** - 0 correct, 0 incorrect\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestLeaderboardText_CutsRowsWithHiddenCount(t *testing.T) {
	row := len("1. **A** - 0 correct, 0 incorrect\n")
	got := leaderboardText(leaderboardUsers(10), "", 3*row+len("-# 7 more not shown\n"))
	want := "1. **A** - 0 correct, 0 incorrect\n2. **B** - 0 correct, 0 incorrect\n3. **C** - 0 correct, 0 incorrect\n-# 7 more not shown\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestLeaderboardText_KeepsCallerRow(t *testing.T) {
	row := len("1. **A** - 0 correct, 0 incorrect\n")
	got := leaderboardText(leaderboardUsers(10), "h", 3*row+len("-# 6 more not shown\n"))
	want := "1. **A** - 0 correct, 0 incorrect\n2. **B** - 0 correct, 0 incorrect\n8. **H** - 0 correct, 0 incorrect\n-# 7 more not shown\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestLeaderboardText_CallerAlreadyShown(t *testing.T) {
	got := leaderboardText(leaderboardUsers(10), "a", 200)
	if strings.Count(got, "**A**") != 1 {
		t.Errorf("caller row repeated: %q", got)
	}
}
