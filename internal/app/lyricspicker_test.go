package app

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"mygo-music/internal/music"
	"mygo-music/internal/netease"
)

func TestLyricsPickerSearchPreviewApplyAndReset(t *testing.T) {
	origSearch := neteaseSearchLyrics
	origFetch := neteaseFetchLyrics
	defer func() {
		neteaseSearchLyrics = origSearch
		neteaseFetchLyrics = origFetch
	}()

	neteaseSearchLyrics = func(query string, limit int) []netease.MatchResult {
		return []netease.MatchResult{
			{SongID: 101, Title: "Wrong Version", Artist: "Other", Album: "Cover Album", Duration: 180 * time.Second},
			{SongID: 202, Title: "晴天", Artist: "周杰伦", Album: "叶惠美", Duration: 269 * time.Second, CoverURL: "https://example.com/c.jpg"},
		}
	}
	neteaseFetchLyrics = func(songID int64) (string, string) {
		if songID == 202 {
			return "[00:05.00]故事的小黄花\n[00:10.00]从出生那年就飘着", "[00:05.00]Little yellow flower"
		}
		return "[00:01.00]Wrong lyric", ""
	}

	a := newTestApp()
	ch := posts(a)
	a.lyricsChoices = loadLyricsStore()
	a.pl.CustomLyrics = a.lyricsChoices.get

	tr := music.Track{
		ID:      "yt-123",
		Source:  "youtube-music",
		Name:    "晴天 (Official Music Video)",
		Artists: []music.Artist{{Name: "周杰伦 - Topic"}},
	}
	a.pl.Cue([]music.Track{tr})

	// Open picker -> auto-suggests clean query and searches
	a.openLyricsPicker(&tr)
	if !a.lyricsPick.open {
		t.Fatal("expected lyrics picker dialog to open")
	}
	if a.lyricsPick.query != "晴天 周杰伦" {
		t.Fatalf("expected clean suggested query %q, got %q", "晴天 周杰伦", a.lyricsPick.query)
	}
	until(t, ch, func() bool { return !a.lyricsPick.searching && !a.lyricsPick.previewBusy })

	if len(a.lyricsPick.results) != 2 || a.lyricsPick.selected != 0 {
		t.Fatalf("unexpected search/preview state: results=%d selected=%d", len(a.lyricsPick.results), a.lyricsPick.selected)
	}

	// Render dialog frame and select second candidate
	tt := ui.NewTester(a.view, 1200, 800)
	tt.Frame()
	a.previewPickedLyric(1)
	until(t, ch, func() bool { return !a.lyricsPick.previewBusy })
	if a.lyricsPick.selected != 1 || len(a.lyricsPick.previewRows) != 2 {
		t.Fatalf("expected preview rows for candidate 1, got %d", len(a.lyricsPick.previewRows))
	}

	// Apply selected lyric
	a.applyPickedLyric()
	if a.lyricsPick.open {
		t.Fatal("expected picker dialog to close after apply")
	}
	lines, loaded := a.pl.Lyrics()
	if !loaded || len(lines) != 2 || lines[0].Text != "故事的小黄花" || lines[0].Translation != "Little yellow flower" {
		t.Fatalf("unexpected applied lyrics: loaded=%v lines=%+v", loaded, lines)
	}
	if got := a.lyricsChoices.get("youtube-music", "yt-123"); got == nil || got.LRC == "" {
		t.Fatal("expected saved lyric override to persist in store")
	}

	// Reload store from disk to verify JSON persistence
	reloaded := loadLyricsStore()
	if !reloaded.has("youtube-music", "yt-123") {
		t.Fatal("expected persisted choice on disk")
	}

	// Reset to default clears the persisted choice
	a.openLyricsPicker(&tr)
	until(t, ch, func() bool { return !a.lyricsPick.searching && !a.lyricsPick.previewBusy })
	a.resetPickedLyric()
	if a.lyricsChoices.has("youtube-music", "yt-123") {
		t.Fatal("expected saved choice to be removed after reset")
	}
}
