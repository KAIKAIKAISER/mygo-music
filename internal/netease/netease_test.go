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

func TestSuggestQueryAndSearch(t *testing.T) {
	if got := SuggestQuery("Taylor Swift - Topic", "Cruel Summer (Official Music Video)"); got != "Cruel Summer Taylor Swift" {
		t.Errorf("SuggestQuery got %q, want %q", got, "Cruel Summer Taylor Swift")
	}
	if got := SuggestQuery("", "米津玄師 - Lemon [MV]"); got != "Lemon 米津玄師" {
		t.Errorf("SuggestQuery got %q, want %q", got, "Lemon 米津玄師")
	}

	results := Search("晴天 周杰伦", 5)
	if len(results) == 0 {
		t.Fatal("expected search results, got 0")
	}
	if results[0].SongID <= 0 || results[0].Title == "" {
		t.Errorf("invalid search result: %+v", results[0])
	}
	lrc, _ := FetchLyrics(results[0].SongID)
	if lrc == "" {
		t.Errorf("expected non-empty lyrics for song id %d", results[0].SongID)
	}
}
