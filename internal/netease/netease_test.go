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

func TestMatchYouTubeNoise(t *testing.T) {
	res := Match("YOASOBI", "夜に駆ける (Official Music Video)")
	if res == nil {
		t.Fatal("expected match for youtube title, got nil")
	}
	if res.LyricsLRC == "" {
		t.Error("expected non-empty lyrics LRC for YOASOBI")
	}
}

func TestMatchTopicArtist(t *testing.T) {
	res := Match("Taylor Swift - Topic", "Cruel Summer")
	if res == nil {
		t.Fatal("expected match for topic artist, got nil")
	}
	if res.LyricsLRC == "" {
		t.Error("expected non-empty lyrics LRC for Taylor Swift")
	}
}
