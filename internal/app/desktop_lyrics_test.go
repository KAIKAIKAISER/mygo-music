package app

import (
	"testing"
	"time"

	"github.com/egoist/mygo"
)

func TestDesktopLyricsRightEdgeAnchorPreservedAcrossLongAndShortLyrics(t *testing.T) {
	area := mygo.Rectangle{X: 0, Y: 0, Width: 1920, Height: 1032}

	// Simulate user dragging a 380px window close to the right edge (10px margin)
	dragged := mygo.Rectangle{X: 1530, Y: 880, Width: 380, Height: 120}
	mode, rGap, lGap, cx := classifyDragAnchor(area, dragged, 160)
	if mode != anchorRight {
		t.Fatalf("expected anchorRight, got %q", mode)
	}
	if rGap != 10 {
		t.Fatalf("expected rightGap 10, got %v", rGap)
	}
	if lGap != 0 {
		t.Fatalf("expected leftGap 0, got %v", lGap)
	}

	// Long lyric expands leftward to 1000px while keeping right edge pinned at 1910
	longW, longH := float32(1000), float32(120)
	longX, _ := computeLyricsOrigin(area, mode, rGap, lGap, cx, 880, longW, longH)
	if longX+int(longW) != 1910 {
		t.Fatalf("long lyric right edge drifted: X=%d W=%d (right=%d, want 1910)", longX, int(longW), longX+int(longW))
	}

	// Programmatic SetBounds move notification must be ignored so anchor doesn't shift
	d := &desktopLyrics{
		w:               longW,
		h:               longH,
		anchorMode:      mode,
		anchorRightGap:  rGap,
		expectedBounds:  mygo.Rectangle{X: longX, Y: 880, Width: int(longW), Height: int(longH)},
		ignoreMoveUntil: time.Now().Add(400 * time.Millisecond),
	}
	if !d.isProgrammaticMove(mygo.Rectangle{X: longX, Y: 880, Width: int(longW), Height: int(longH)}) {
		t.Fatal("expected programmatic move from SetBounds to be ignored")
	}

	// Next song has a short lyric (128px): right edge MUST stay at 1910 (X=1782), not jump left
	shortW, shortH := float32(128), float32(120)
	shortX, _ := computeLyricsOrigin(area, d.anchorMode, d.anchorRightGap, 0, cx, 880, shortW, shortH)
	if shortX != 1782 || shortX+int(shortW) != 1910 {
		t.Fatalf("short lyric did not hug right edge: X=%d W=%d (right=%d, want 1782..1910)", shortX, int(shortW), shortX+int(shortW))
	}
}

func TestDesktopLyricsMaxWidthRespectsRightAnchor(t *testing.T) {
	area := mygo.Rectangle{X: 0, Y: 0, Width: 1920, Height: 1032}
	maxW := computeLyricsMaxWidth(area, anchorRight, 10, 0, 128)
	// rightEdge = 1910, left margin = 8 -> maxW = 1902
	if maxW != 1902 {
		t.Fatalf("expected maxW 1902, got %v", maxW)
	}
	x, _ := computeLyricsOrigin(area, anchorRight, 10, 0, 0, 880, maxW, 120)
	if x != 8 || x+int(maxW) != 1910 {
		t.Fatalf("expected clamped bounds 8..1910, got %d..%d", x, x+int(maxW))
	}
}

func TestDesktopLyricsCenterAnchorSurvivesEdgeClamping(t *testing.T) {
	area := mygo.Rectangle{X: 0, Y: 0, Width: 1920, Height: 1032}
	dragged := mygo.Rectangle{X: 760, Y: 880, Width: 400, Height: 120}
	mode, _, _, cx := classifyDragAnchor(area, dragged, 160)
	if mode != anchorCenter || cx != 960 {
		t.Fatalf("expected anchorCenter at 960, got %q at %v", mode, cx)
	}

	// Very wide lyric stays centered within screen
	wideX, _ := computeLyricsOrigin(area, mode, 0, 0, cx, 880, 1800, 120)
	if wideX != 60 {
		t.Fatalf("expected wide lyric at X=60, got %d", wideX)
	}

	// Subsequent short lyric returns to exact center 960
	shortX, _ := computeLyricsOrigin(area, mode, 0, 0, cx, 880, 160, 120)
	if shortX != 880 {
		t.Fatalf("expected short lyric centered at X=880, got %d", shortX)
	}
}

func TestDesktopLyricsLeftEdgeAnchor(t *testing.T) {
	area := mygo.Rectangle{X: 100, Y: 0, Width: 1920, Height: 1032}
	dragged := mygo.Rectangle{X: 112, Y: 880, Width: 300, Height: 120}
	mode, _, lGap, cx := classifyDragAnchor(area, dragged, 160)
	if mode != anchorLeft || lGap != 12 {
		t.Fatalf("expected anchorLeft with gap 12, got %q with %v", mode, lGap)
	}
	shortX, _ := computeLyricsOrigin(area, mode, 0, lGap, cx, 880, 140, 120)
	if shortX != 112 {
		t.Fatalf("expected left-anchored lyric at X=112, got %d", shortX)
	}
}
