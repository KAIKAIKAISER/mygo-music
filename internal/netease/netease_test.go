package netease

import (
	"testing"
)

func TestMatch(t *testing.T) {
	res := Match("Imagine Dragons", "Radioactive")
	if res == nil {
		t.Fatal("expected match, got nil")
	}
	if res.SongID <= 0 {
		t.Errorf("expected positive song id, got %d", res.SongID)
	}
	if res.CoverURL == "" {
		t.Error("expected non-empty cover url")
	}
	if res.LyricsLRC == "" {
		t.Error("expected non-empty lyrics LRC")
	}
}
