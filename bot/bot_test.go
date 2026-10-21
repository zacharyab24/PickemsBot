package bot

import "testing"

func TestNewBot_EmptyToken_Errors(t *testing.T) {
	if _, err := NewBot("", nil, nil, ""); err == nil {
		t.Fatal("expected an error for an empty bot token")
	}
}
