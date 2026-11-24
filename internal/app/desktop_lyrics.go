package app

import (
	"fmt"
	"math"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"mygo-music/internal/lyrics"
)

const (
	anchorCenter = "center"
	anchorLeft   = "left"
	anchorRight  = "right"
)

var lyricsAnchors = []struct {
	id    string
	label string
}{
	{anchorCenter, "Anchor Center"},
	{anchorRight, "Anchor Right"},
	{anchorLeft, "Anchor Left"},
}

type lyricsTheme struct {
	id    string
	name  string
	color ui.Color
}

var lyricsThemes = []lyricsTheme{
	{"gold", "Sunset Gold", ui.RGB(255, 184, 0)},   // NetEase Gold #FFB800
	{"cyan", "Aurora Cyan", ui.RGB(0, 226, 255)},   // Aurora Cyan #00E2FF
	{"red", "Vibrant Red", ui.RGB(255, 75, 75)},    // Vibrant Red #FF4B4B
	{"green", "Fresh Green", ui.RGB(38, 222, 129)}, // Fresh Green #26DE81
	{"pink", "Sakura Pink", ui.RGB(255, 107, 129)}, // Sakura Pink #FF6B81
	{"white", "Pure White", ui.RGB(255, 255, 255)}, // Pure White #FFFFFF
}

type fontOption struct {
	id     string
	label  string
	family string
}

var lyricsFonts = []fontOption{
	{"default", "Default (consistent with client)", ""},
	{"yahei", "Microsoft YaHei", "Microsoft YaHei, PingFang SC, sans-serif"},
	{"pingfang", "PingFang SC", "PingFang SC, Hiragino Sans GB, sans-serif"},
	{"simhei", "SimHei", "SimHei, STHeiti, sans-serif"},
	{"kaiti", "KaiTi", "KaiTi, STKaiti, serif"},
	{"simsun", "SimSun", "SimSun, STSong, serif"},
	{"harmony", "HarmonyOS Sans", "HarmonyOS Sans SC, sans-serif"},
}

var lyricsFontSizes = []struct {
	size  float32
	label string
}{
	{24, "Compact (24px)"},
	{28, "Medium (28px)"},
	{32, "Standard (32px)"},
	{38, "Large (38px)"},
	{44, "Extra Large (44px)"},
}

func getLyricsColor(id string) ui.Color {
	for _, t := range lyricsThemes {
		if t.id == id {
			return t.color
		}
	}
	return ui.RGB(255, 184, 0)
}

func getFontFamily(id string) string {
	for _, f := range lyricsFonts {
		if f.id == id {
			return f.family
		}
	}
	return ""
}

// desktopLyrics represents the floating desktop lyrics window.
type desktopLyrics struct {
	a   *app
	win *mygo.Window

	w, h           float32
	anchorMode     string
	anchorRightGap float32
	anchorLeftGap  float32
	centerX        float32
	screenY        float32

	locked       bool
	mouseInside  bool
	overUnlock   bool
	toolbarHover bool

	updatingBounds  bool
	ignoreMoveUntil time.Time
	expectedBounds  mygo.Rectangle
	saveTimer       *time.Timer

	lastLine1        string
	lastLine2        string
	lastFontSize     float32
	lastFontFamily   string
	lastAnchor       string
	lastToolbarHover bool
	targetW          float32
	targetH          float32
}

func displayWorkArea(disp mygo.Display) mygo.Rectangle {
	area := disp.WorkArea
	if area.Width <= 0 || area.Height <= 0 {
		area = disp.Bounds
	}
	if area.Width <= 0 || area.Height <= 0 {
		area = mygo.Rectangle{X: 0, Y: 0, Width: 1920, Height: 1080}
	}
	return area
}

func clampFloat(v, minV, maxV float32) float32 {
	if maxV < minV {
		return minV
	}
	if v < minV {
		return minV
	}
	if v > maxV {
		return maxV
	}
	return v
}

// classifyDragAnchor determines the anchor mode and gap from a manually dragged window bounds.
// Near the right or left screen edge (within 180px), it locks to that edge so shrinking/growing
// lyrics stay pinned flush against the edge instead of drifting toward the center.
func classifyDragAnchor(area, b mygo.Rectangle, edgeSnapThreshold float32) (mode string, rightGap, leftGap, centerX float32) {
	lGap := float32(b.X - area.X)
	rGap := float32(area.X+area.Width) - float32(b.X+b.Width)

	if rGap <= edgeSnapThreshold && rGap < lGap {
		return anchorRight, max(0, rGap), 0, float32(b.X) + float32(b.Width)/2
	}
	if lGap <= edgeSnapThreshold && lGap <= rGap {
		return anchorLeft, 0, max(0, lGap), float32(b.X) + float32(b.Width)/2
	}
	return anchorCenter, 0, 0, float32(b.X) + float32(b.Width)/2
}

// computeLyricsMaxWidth computes the maximum window width allowed from the current anchor
// before reaching the opposite edge of the work area.
func computeLyricsMaxWidth(area mygo.Rectangle, anchor string, rightGap, leftGap, minWinW float32) float32 {
	var maxW float32
	switch anchor {
	case anchorRight:
		gap := clampFloat(rightGap, 0, float32(area.Width)/2)
		rightEdge := float32(area.X+area.Width) - gap
		maxW = rightEdge - float32(area.X+8)
	case anchorLeft:
		gap := clampFloat(leftGap, 0, float32(area.Width)/2)
		leftEdge := float32(area.X) + gap
		maxW = float32(area.X+area.Width-8) - leftEdge
	default:
		maxW = float32(area.Width - 16)
	}
	if float32(area.Width-16) < minWinW {
		return float32(max(100, area.Width-8))
	}
	if maxW < minWinW {
		return minWinW
	}
	return maxW
}

// computeLyricsOrigin calculates the window's top-left X, Y coordinates according to its anchor mode.
func computeLyricsOrigin(area mygo.Rectangle, anchor string, rightGap, leftGap, centerX, screenY, targetW, targetH float32) (int, int) {
	var x float32
	switch anchor {
	case anchorRight:
		rightEdge := float32(area.X+area.Width) - max(0, rightGap)
		x = rightEdge - targetW
	case anchorLeft:
		x = float32(area.X) + max(0, leftGap)
	default:
		if centerX <= 0 {
			centerX = float32(area.X) + float32(area.Width)/2
		}
		x = centerX - targetW/2
	}

	newX := int(math.Round(float64(x)))
	newY := int(math.Round(float64(screenY)))

	minX := area.X + 4
	maxX := area.X + area.Width - 4 - int(targetW)
	if maxX < minX {
		maxX = minX
	}
	if newX > maxX {
		newX = maxX
	}
	if newX < minX {
		newX = minX
	}

	minY := area.Y + 4
	maxY := area.Y + area.Height - 4 - int(targetH)
	if maxY < minY {
		maxY = minY
	}
	if newY > maxY {
		newY = maxY
	}
	if newY < minY {
		newY = minY
	}

	return newX, newY
}

func newDesktopLyrics(a *app) *desktopLyrics {
	d := &desktopLyrics{a: a}

	disp := mygo.Screen.PrimaryDisplay()
	area := displayWorkArea(disp)
	defaultW := 220
	defaultH := 120
	d.w = float32(defaultW)
	d.h = float32(defaultH)
	d.centerX = float32(area.X + area.Width/2)
	d.screenY = float32(area.Y + area.Height - defaultH - 70)
	d.anchorMode = anchorCenter

	d.win = mygo.NewWindow(mygo.WindowOptions{
		Title:           Tf("%s Desktop Lyrics", mygo.App.Name()),
		Hidden:          true,
		Frameless:       true,
		Transparent:     true,
		BackgroundColor: "transparent",
		AlwaysOnTop:     true,
		DisableShadow:   true,
		DisableResize:   true,
		SkipTaskbar:     true,
		StateKey:        "desktop-lyrics",
		X:               int(d.centerX - d.w/2),
		Y:               int(d.screenY),
		Width:           int(d.w),
		Height:          int(d.h),
		Content:         ui.View(d.view),
	})

	initB := d.win.Bounds()
	if initB.Width > 0 && initB.Height > 0 {
		disp = mygo.Screen.DisplayMatching(initB)
		area = displayWorkArea(disp)
		d.screenY = float32(initB.Y)
		d.w = float32(initB.Width)
		d.h = float32(initB.Height)
	}

	if a.settings.DesktopLyricsAnchor != "" {
		d.anchorMode = a.settings.DesktopLyricsAnchor
		d.anchorRightGap = a.settings.DesktopLyricsRightGap
		d.anchorLeftGap = a.settings.DesktopLyricsLeftGap
		d.centerX = a.settings.DesktopLyricsCenterX
		if d.anchorMode == anchorCenter && d.centerX <= 0 {
			if initB.Width > 0 {
				d.centerX = float32(initB.X) + float32(initB.Width)/2
			} else {
				d.centerX = float32(area.X + area.Width/2)
			}
		}
	} else if initB.Width > 0 {
		// Migrate legacy window-state position: if it sat in the right/left edge zone,
		// anchor cleanly to the edge so lyrics never drift away after long lines.
		mode, _, _, cx := classifyDragAnchor(area, initB, 200)
		d.anchorMode = mode
		switch mode {
		case anchorRight:
			d.anchorRightGap = 8
		case anchorLeft:
			d.anchorLeftGap = 8
		default:
			d.centerX = cx
		}
		a.settings.DesktopLyricsAnchor = d.anchorMode
		a.settings.DesktopLyricsRightGap = d.anchorRightGap
		a.settings.DesktopLyricsLeftGap = d.anchorLeftGap
		a.settings.DesktopLyricsCenterX = d.centerX
		a.settings.save()
	}

	d.expectedBounds = initB
	removeWindowBorder(d.win.NativeHandle())

	d.win.OnShow(func() {
		removeWindowBorder(d.win.NativeHandle())
	})

	d.win.OnMove(func() {
		b := d.win.Bounds()
		if b.Width <= 0 || d.isProgrammaticMove(b) {
			return
		}
		d.updateAnchorFromDrag(b)
	})

	d.win.OnClose(func(e *mygo.CloseEvent) {
		if !a.quitting {
			e.PreventDefault()
			a.setDesktopLyrics(false)
		}
	})

	if a.settings.DesktopLyrics {
		removeWindowBorder(d.win.NativeHandle())
		d.win.ShowInactive()
		d.applyLock(a.settings.DesktopLyricsLocked)
	}

	return d
}

func (d *desktopLyrics) effectiveAnchor() string {
	switch d.anchorMode {
	case anchorRight, anchorLeft, anchorCenter:
		return d.anchorMode
	default:
		return anchorCenter
	}
}

func (d *desktopLyrics) isProgrammaticMove(b mygo.Rectangle) bool {
	if d.updatingBounds {
		return true
	}
	if time.Now().Before(d.ignoreMoveUntil) {
		dx := math.Abs(float64(b.X - d.expectedBounds.X))
		dy := math.Abs(float64(b.Y - d.expectedBounds.Y))
		if dx <= 4 && dy <= 4 {
			return true
		}
		if b.Width != int(math.Round(float64(d.w))) || b.Height != int(math.Round(float64(d.h))) {
			return true
		}
	}
	return false
}

func (d *desktopLyrics) updateAnchorFromDrag(b mygo.Rectangle) {
	disp := mygo.Screen.DisplayMatching(b)
	area := displayWorkArea(disp)
	mode, rGap, lGap, cx := classifyDragAnchor(area, b, 160)

	d.anchorMode = mode
	d.anchorRightGap = rGap
	d.anchorLeftGap = lGap
	d.centerX = cx
	d.screenY = float32(b.Y)
	d.expectedBounds = b
	d.scheduleSaveAnchor()
}

func (d *desktopLyrics) scheduleSaveAnchor() {
	if d.saveTimer != nil {
		d.saveTimer.Stop()
	}
	mode := d.anchorMode
	rGap := d.anchorRightGap
	lGap := d.anchorLeftGap
	cx := d.centerX
	d.saveTimer = time.AfterFunc(350*time.Millisecond, func() {
		if d.a != nil {
			d.a.post(func() {
				d.a.settings.DesktopLyricsAnchor = mode
				d.a.settings.DesktopLyricsRightGap = rGap
				d.a.settings.DesktopLyricsLeftGap = lGap
				d.a.settings.DesktopLyricsCenterX = cx
				d.a.settings.save()
			})
		}
	})
}

func (d *desktopLyrics) show() {
	if d.win != nil {
		removeWindowBorder(d.win.NativeHandle())
		d.win.ShowInactive()
		d.applyLock(d.a.settings.DesktopLyricsLocked)
	}
}

func (d *desktopLyrics) hide() {
	if d.win != nil {
		d.win.Hide()
	}
}

func (d *desktopLyrics) applyLock(locked bool) {
	d.locked = locked
	d.mouseInside = false
	d.overUnlock = false
	d.toolbarHover = false
	if d.win != nil {
		d.win.SetIgnoreMouseEvents(locked)
		d.win.Invalidate()
	}
}

func (d *desktopLyrics) redraw() {
	if d.win != nil && d.win.IsVisible() {
		d.win.Invalidate()
	}
}

func (d *desktopLyrics) view(c *ui.Context) {
	c.Root().Background(ui.Transparent)
	a := d.a
	pl := a.pl
	tr := pl.Current()
	lines, _ := pl.Lyrics()
	pos := pl.Position()
	cur := -1
	if len(lines) > 0 {
		cur = lyrics.Index(lines, pos)
	}

	// Line 1: Current singing line
	line1Text := ""
	var currentWords []lyrics.Word
	if cur >= 0 && cur < len(lines) {
		line1Text = lines[cur].Text
		currentWords = lines[cur].Words
	} else if tr != nil {
		line1Text = tr.Name
	} else {
		line1Text = mygo.App.Name()
	}

	// Line 2: Translation or next upcoming line
	line2Text := ""
	if cur >= 0 && cur < len(lines) {
		if lines[cur].Translation != "" {
			line2Text = lines[cur].Translation
		} else if cur+1 < len(lines) {
			line2Text = lines[cur+1].Text
		}
	} else if tr != nil {
		line2Text = tr.ArtistNames()
	}

	locked := a.settings.DesktopLyricsLocked
	fontSize := a.desktopLyricsFontSize()
	fontFamily := getFontFamily(a.settings.DesktopLyricsFont)
	activeColor := getLyricsColor(a.settings.DesktopLyricsColor)
	secondaryColor := ui.RGBA(255, 255, 255, 0.78)
	anchor := d.effectiveAnchor()

	b := d.win.Bounds()
	disp := mygo.Screen.DisplayMatching(b)
	area := displayWorkArea(disp)

	// Cursor hover tracking (both locked unlock-button and unlocked auto-expanding toolbar)
	if d.win != nil && d.win.IsVisible() {
		pt := mygo.Screen.CursorScreenPoint()
		inside := pt.X >= b.X && pt.X < b.X+b.Width && pt.Y >= b.Y && pt.Y < b.Y+b.Height
		if locked {
			c.After(50 * time.Millisecond)
			btnW, btnH := min(max(100, b.Width-16), 132), 36
			var btnX int
			switch anchor {
			case anchorRight:
				btnX = b.X + b.Width - 12 - btnW
			case anchorLeft:
				btnX = b.X + 12
			default:
				btnX = b.X + (b.Width-btnW)/2
			}
			btnY := b.Y + 2
			overBtn := inside && pt.X >= btnX && pt.X <= btnX+btnW && pt.Y >= btnY && pt.Y <= btnY+btnH

			if inside != d.mouseInside || overBtn != d.overUnlock {
				d.mouseInside = inside
				d.overUnlock = overBtn
				d.win.SetIgnoreMouseEvents(!overBtn)
				d.win.Invalidate()
			}
		} else {
			d.mouseInside = false
			d.overUnlock = false
			if inside != d.toolbarHover {
				d.toolbarHover = inside
			}
			if inside {
				c.After(80 * time.Millisecond)
			}
		}
	}

	// Schedule redraw while playing; if paused, periodically check hover
	if pl.Playing() {
		if cur >= 0 && cur < len(lines) && len(lines[cur].Words) > 0 {
			c.After(40 * time.Millisecond)
		} else if cur+1 < len(lines) {
			diff := lines[cur+1].At - pos + 20*time.Millisecond
			if diff > 100*time.Millisecond {
				diff = 100 * time.Millisecond
			}
			c.After(max(30*time.Millisecond, diff))
		} else {
			c.After(100 * time.Millisecond)
		}
	} else {
		c.After(250 * time.Millisecond)
	}

	// Minimum width stays compact when toolbar isn't shown so short lyrics tightly hug the screen edge
	padX := float32(12)
	strokeBuf := float32(10)
	minWinW := float32(128)
	if !locked && d.toolbarHover {
		minWinW = 396
	}
	maxWinW := computeLyricsMaxWidth(area, anchor, d.anchorRightGap, d.anchorLeftGap, minWinW)

	// 1. Measure natural single-line width of both lines
	baseSize2 := max(15, fontSize*0.72)
	w1, h1Natural := measureTextSize(c, line1Text, fontFamily, fontSize, 800, 0)
	w2, h2Natural := float32(0), float32(0)
	if line2Text != "" {
		w2, h2Natural = measureTextSize(c, line2Text, fontFamily, baseSize2, 600, 0)
	}

	// 2. Tight adaptive width bounded by anchor-aware maxWinW
	contentW := max(w1, w2)
	neededW := max(minWinW, contentW+padX*2+strokeBuf)
	targetW := min(neededW, maxWinW)
	usableW := max(80, targetW-padX*2)

	// 3. Adaptive font downscale or wrapping when text exceeds usableW
	line1FontSize := fontSize
	line2FontSize := baseSize2
	wrapLimit1 := max(80, usableW-4)
	wrapLimit2 := max(80, usableW-4)

	line1WrappedH := h1Natural
	if w1 > wrapLimit1 && w1 > 0 {
		if w1 <= wrapLimit1*1.22 {
			scale := wrapLimit1 / w1
			line1FontSize = max(18.0, fontSize*scale)
		} else {
			line1FontSize = max(20.0, fontSize*0.88)
			_, line1WrappedH = measureTextSize(c, line1Text, fontFamily, line1FontSize, 800, usableW)
		}
	}

	line2WrappedH := h2Natural
	if w2 > wrapLimit2 && w2 > 0 {
		if w2 <= wrapLimit2*1.22 {
			scale := wrapLimit2 / w2
			line2FontSize = max(13.0, baseSize2*scale)
		} else {
			line2FontSize = max(14.0, baseSize2*0.9)
			_, line2WrappedH = measureTextSize(c, line2Text, fontFamily, line2FontSize, 600, usableW)
		}
	}

	// 4. Dynamic target height
	totalNeededH := 28 + line1WrappedH + 14
	if line2Text != "" {
		totalNeededH += line2WrappedH + 6
	}
	targetH := max(116.0, totalNeededH+8)
	if targetH > float32(area.Height-16) {
		targetH = float32(area.Height - 16)
	}

	// 5. Anchor-preserving window position
	newX, newY := computeLyricsOrigin(area, anchor, d.anchorRightGap, d.anchorLeftGap, d.centerX, d.screenY, targetW, targetH)

	// 6. Update bounds immediately when text/anchor/hover/size shifts
	contentChanged := line1Text != d.lastLine1 ||
		line2Text != d.lastLine2 ||
		fontSize != d.lastFontSize ||
		fontFamily != d.lastFontFamily ||
		anchor != d.lastAnchor ||
		d.toolbarHover != d.lastToolbarHover
	sizeShift := math.Abs(float64(float32(b.Width)-targetW)) >= 2 || math.Abs(float64(float32(b.Height)-targetH)) >= 2
	posShift := math.Abs(float64(b.X-newX)) >= 2 || math.Abs(float64(b.Y-newY)) >= 2

	if contentChanged || sizeShift || posShift {
		d.lastLine1 = line1Text
		d.lastLine2 = line2Text
		d.lastFontSize = fontSize
		d.lastFontFamily = fontFamily
		d.lastAnchor = anchor
		d.lastToolbarHover = d.toolbarHover
		d.targetW = targetW
		d.targetH = targetH
		d.w = targetW
		d.h = targetH

		newBounds := mygo.Rectangle{
			X:      newX,
			Y:      newY,
			Width:  int(math.Round(float64(targetW))),
			Height: int(math.Round(float64(targetH))),
		}
		if newBounds != b {
			d.updatingBounds = true
			d.expectedBounds = newBounds
			d.ignoreMoveUntil = time.Now().Add(400 * time.Millisecond)
			d.win.SetBounds(newBounds)
			b = d.win.Bounds()
			d.expectedBounds = b
			d.updatingBounds = false
		}
	}

	var align ui.Align
	switch anchor {
	case anchorRight:
		align = ui.End
	case anchorLeft:
		align = ui.Start
	default:
		align = ui.Center
	}

	wrap := ui.Box(c.Key("desktop-wrap")).Fill().Padding(4, padX).Background(ui.Transparent)
	if !locked {
		wrap.DragWindow()
	}

	wrap.Children(func() {
		ui.Column(c).Fill().Justify(ui.SpaceBetween).Children(func() {
			// Top row: hover toolbar when unlocked, or unlock button when locked and mouse is over lyrics
			if !locked {
				if d.toolbarHover || wrap.Hovered() {
					d.toolbar(c, activeColor, align)
				} else {
					ui.Box(c).Height(28)
				}
			} else {
				if d.mouseInside {
					ui.Row(c.Key("unlock-wrap")).Height(28).Justify(align).Children(func() {
						btn := ui.ButtonBase(c.Key("unlock-btn")).Height(28).PaddingX(14).Gap(6).Radius(14).
							Background(ui.RGBA(25, 25, 30, 0.94)).
							Border(0.8, ui.RGBA(255, 255, 255, 0.3)).
							Shadow(0, 4, 12, -2, ui.RGBA(0, 0, 0, 0.45)).
							AlignItems(ui.Center).FocusRing(false).Label(T("Unlock Desktop Lyrics"))
						if d.overUnlock || btn.Hovered() {
							btn.Background(ui.RGBA(40, 40, 50, 0.98)).Border(0.8, activeColor)
						}
						if btn.Clicked() {
							a.setDesktopLyricsLocked(false)
						}
						btn.Children(func() {
							ui.Icon(c, iconUnlock).Size(14, 14).TextColor(activeColor)
							ui.Text(c, T("Unlock Lyrics")).FontSize(12).FontWeight(600).TextColor(ui.Hex("#ffffff"))
						})
					})
				} else {
					ui.Box(c).Height(28)
				}
			}

			// Lyrics container aligned to anchor so short/long lyrics stay anchored cleanly
			ui.Column(c.Key("lyrics-container")).Grow(1).Justify(ui.Center).AlignItems(align).Gap(6).Children(func() {
				if len(currentWords) > 0 && cur >= 0 && cur < len(lines) {
					rt := ui.RichText(c.Key("rt1")).Width(usableW).TextAlign(align).
						FontSize(line1FontSize).FontWeight(800).
						LineHeight(1.2).LetterSpacing(-0.2).
						TextStroke(2.2, ui.RGBA(0, 0, 0, 0.8)).
						TextShadow(0, 2, 8, ui.RGBA(0, 0, 0, 0.95))
					if fontFamily != "" {
						rt.Font(fontFamily)
					}
					rt.Children(func() {
						for wi, wd := range currentWords {
							lit := clamp01(float32((pos - wd.Start).Seconds() / max(0.05, (wd.End-wd.Start).Seconds())))
							col := activeColor.Alpha(0.45 + 0.55*lit)
							ui.Text(c.Key(wi), wd.Text).TextColor(col)
						}
					})
				} else {
					el := ui.Text(c.Key("txt1"), line1Text).Width(usableW).TextAlign(align).
						FontSize(line1FontSize).FontWeight(800).
						LineHeight(1.2).LetterSpacing(-0.2).
						TextColor(activeColor).
						TextStroke(2.2, ui.RGBA(0, 0, 0, 0.8)).
						TextShadow(0, 2, 8, ui.RGBA(0, 0, 0, 0.95))
					if fontFamily != "" {
						el.Font(fontFamily)
					}
				}

				if line2Text != "" {
					el := ui.Text(c.Key("txt2"), line2Text).Width(usableW).TextAlign(align).
						FontSize(line2FontSize).FontWeight(600).
						LineHeight(1.2).LetterSpacing(-0.1).
						TextColor(secondaryColor).
						TextStroke(1.6, ui.RGBA(0, 0, 0, 0.75)).
						TextShadow(0, 2, 6, ui.RGBA(0, 0, 0, 0.85))
					if fontFamily != "" {
						el.Font(fontFamily)
					}
				}
			})

			// Bottom padding spacer
			ui.Box(c).Height(4)
		})
	})
}

func measureTextSize(c *ui.Context, text, font string, size float32, weight int, wrapWidth float32) (float32, float32) {
	if text == "" || size <= 0 {
		return 0, 0
	}
	w, h := c.MeasureText(wrapWidth, ui.Span{
		Text:   text,
		Font:   font,
		Size:   size,
		Weight: weight,
	})
	if w <= 0 {
		w = estimateTextWidth(text, size)
	}
	if h <= 0 {
		h = size * 1.25
	}
	return w, h
}

func estimateTextWidth(text string, fontSize float32) float32 {
	if text == "" || fontSize <= 0 {
		return 0
	}
	var total float32
	for _, r := range text {
		switch {
		case r > 0x2e80:
			total += fontSize * 1.05
		case r == ' ' || r == '\t':
			total += fontSize * 0.35
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			if r == 'i' || r == 'l' || r == 'j' || r == 'f' || r == 't' || r == 'r' {
				total += fontSize * 0.35
			} else if r == 'm' || r == 'w' {
				total += fontSize * 0.8
			} else {
				total += fontSize * 0.55
			}
		case r >= 'A' && r <= 'Z':
			if r == 'I' || r == 'J' {
				total += fontSize * 0.4
			} else if r == 'M' || r == 'W' {
				total += fontSize * 0.9
			} else {
				total += fontSize * 0.7
			}
		case r == '.' || r == ',' || r == '!' || r == '?' || r == ':' || r == ';' || r == '\'' || r == '"':
			total += fontSize * 0.35
		case r == '-' || r == '_' || r == '/' || r == '(' || r == ')' || r == '[' || r == ']':
			total += fontSize * 0.45
		default:
			if r > 0x7f {
				total += fontSize * 0.9
			} else {
				total += fontSize * 0.55
			}
		}
	}
	return total + fontSize*0.2
}

func (d *desktopLyrics) toolbar(c *ui.Context, activeColor ui.Color, justify ui.Align) {
	a := d.a
	pl := a.pl

	ui.Row(c.Key("tb-wrap")).Justify(justify).Children(func() {
		ui.Row(c.Key("tb")).Height(28).PaddingX(10).Gap(6).Radius(14).
			Background(ui.RGBA(25, 25, 30, 0.92)).
			Border(0.5, ui.RGBA(255, 255, 255, 0.2)).
			Shadow(0, 4, 12, -2, ui.RGBA(0, 0, 0, 0.35)).
			AlignItems(ui.Center).Children(func() {

			// Drag handle
			ui.Row(c.Key("drag-handle")).DragWindow().AlignItems(ui.Center).Gap(3).PaddingX(4).Children(func() {
				ui.Icon(c, iconMove).Size(13, 13).TextColor(ui.RGBA(255, 255, 255, 0.6))
			})

			// Prev
			if toolIconBtn(c.Key("prev"), iconPrev, 13, ui.RGBA(255, 255, 255, 0.85), T("Previous")).Clicked() {
				pl.Prev()
			}

			// Play / Pause
			playIcon := iconPlay
			if pl.Playing() {
				playIcon = iconPause
			}
			if toolIconBtn(c.Key("play"), playIcon, 13, ui.RGBA(255, 255, 255, 0.95), T("Play/Pause")).Clicked() {
				pl.TogglePlay()
			}

			// Next
			if toolIconBtn(c.Key("next"), iconNext, 13, ui.RGBA(255, 255, 255, 0.85), T("Next")).Clicked() {
				pl.Next()
			}

			// Divider
			ui.Box(c).Width(1).Height(14).Background(ui.RGBA(255, 255, 255, 0.2))

			// Color theme cycle
			colorBtn := toolButtonBase(c.Key("color-btn"), T("Lyrics Color"))
			if colorBtn.Clicked() {
				a.cycleDesktopLyricsColor()
			}
			colorBtn.Children(func() {
				ui.Row(c).AlignItems(ui.Center).Gap(4).Children(func() {
					ui.Box(c).Size(11, 11).Radius(6).Background(activeColor).Border(1, ui.RGBA(255, 255, 255, 0.4))
					ui.Icon(c, iconPalette).Size(13, 13).TextColor(ui.RGBA(255, 255, 255, 0.85))
				})
			})

			// Font size cycle
			sizeBtn := toolButtonBase(c.Key("font-size-btn"), T("Desktop Lyrics Font Size"))
			if sizeBtn.Clicked() {
				a.cycleDesktopLyricsFontSize()
			}
			sizeBtn.Children(func() {
				ui.Row(c).AlignItems(ui.Center).Gap(2).Children(func() {
					ui.Icon(c, iconFontSize).Size(12, 12).TextColor(ui.RGBA(255, 255, 255, 0.85))
					ui.Text(c, fmt.Sprintf("%d", int(a.desktopLyricsFontSize()))).FontSize(11).FontWeight(700).TextColor(ui.RGBA(255, 255, 255, 0.85))
				})
			})

			// Font family cycle
			familyBtn := toolButtonBase(c.Key("font-family-btn"), T("Lyrics Font"))
			if familyBtn.Clicked() {
				a.cycleDesktopLyricsFont()
			}
			familyBtn.Children(func() {
				ui.Text(c, "Aa").FontSize(11).FontWeight(700).TextColor(ui.RGBA(255, 255, 255, 0.85))
			})

			// Anchor alignment cycle (Center / Anchor Right / Anchor Left)
			alignIcon := iconAlignCenter
			alignLabel := T("Anchor Center")
			switch d.effectiveAnchor() {
			case anchorRight:
				alignIcon = iconAlignRight
				alignLabel = T("Anchor Right")
			case anchorLeft:
				alignIcon = iconAlignLeft
				alignLabel = T("Anchor Left")
			}
			if toolIconBtn(c.Key("align-btn"), alignIcon, 13, ui.RGBA(255, 255, 255, 0.85), alignLabel).Clicked() {
				a.cycleDesktopLyricsAnchor()
			}

			// Divider
			ui.Box(c).Width(1).Height(14).Background(ui.RGBA(255, 255, 255, 0.2))

			// Lock button
			if toolIconBtn(c.Key("lock"), iconLock, 13, ui.RGBA(255, 255, 255, 0.85), T("Lock Lyrics (click-through)")).Clicked() {
				a.setDesktopLyricsLocked(true)
			}

			// Close button
			if toolIconBtn(c.Key("close"), iconX, 13, ui.RGBA(255, 255, 255, 0.7), T("Close Desktop Lyrics")).Clicked() {
				a.setDesktopLyrics(false)
			}
		})
	})
}

func toolButtonBase(c *ui.Context, label string) ui.Element {
	b := ui.ButtonBase(c).Height(22).PaddingX(5).Radius(4).AlignItems(ui.Center).FocusRing(false).Label(label)
	fill := ui.Transparent
	if b.Hovered() {
		fill = ui.RGBA(255, 255, 255, 0.15)
	}
	b.Background(fill)
	return b
}

func toolIconBtn(c *ui.Context, icon *ui.SVG, size float32, col ui.Color, label string) ui.Element {
	b := toolButtonBase(c, label)
	b.Children(func() {
		ui.Icon(c, icon).Size(size, size).TextColor(col)
	})
	return b
}

func (a *app) setDesktopLyrics(on bool) {
	a.settings.DesktopLyrics = on
	a.settings.save()
	if a.desktopLyrics != nil {
		if on {
			a.desktopLyrics.show()
		} else {
			a.desktopLyrics.hide()
		}
	}
	a.redraw()
}

func (a *app) toggleDesktopLyrics() {
	a.setDesktopLyrics(!a.settings.DesktopLyrics)
}

func (a *app) setDesktopLyricsLocked(locked bool) {
	a.settings.DesktopLyricsLocked = locked
	a.settings.save()
	if a.desktopLyrics != nil {
		a.desktopLyrics.applyLock(locked)
	}
	a.redraw()
}

func (a *app) toggleDesktopLyricsLock() {
	a.setDesktopLyricsLocked(!a.settings.DesktopLyricsLocked)
}

func (a *app) setDesktopLyricsAnchor(anchor string) {
	switch anchor {
	case anchorRight, anchorLeft, anchorCenter:
	default:
		anchor = anchorCenter
	}

	a.settings.DesktopLyricsAnchor = anchor
	if d := a.desktopLyrics; d != nil {
		d.anchorMode = anchor
		area := displayWorkArea(mygo.Screen.PrimaryDisplay())
		if d.win != nil {
			area = displayWorkArea(mygo.Screen.DisplayMatching(d.win.Bounds()))
		}
		switch anchor {
		case anchorRight:
			d.anchorRightGap = 8
			a.settings.DesktopLyricsRightGap = 8
		case anchorLeft:
			d.anchorLeftGap = 8
			a.settings.DesktopLyricsLeftGap = 8
		default:
			d.centerX = float32(area.X) + float32(area.Width)/2
			a.settings.DesktopLyricsCenterX = d.centerX
		}
		if d.win != nil {
			d.win.Invalidate()
		}
	}
	a.settings.save()
	a.redraw()
}

func (a *app) cycleDesktopLyricsAnchor() {
	cur := anchorCenter
	if a.desktopLyrics != nil {
		cur = a.desktopLyrics.effectiveAnchor()
	} else if a.settings.DesktopLyricsAnchor != "" {
		cur = a.settings.DesktopLyricsAnchor
	}
	switch cur {
	case anchorCenter:
		a.setDesktopLyricsAnchor(anchorRight)
	case anchorRight:
		a.setDesktopLyricsAnchor(anchorLeft)
	default:
		a.setDesktopLyricsAnchor(anchorCenter)
	}
}

func (a *app) cycleDesktopLyricsColor() {
	themes := lyricsThemes
	at := 0
	for i, t := range themes {
		if t.id == a.settings.DesktopLyricsColor {
			at = i
			break
		}
	}
	next := (at + 1) % len(themes)
	a.settings.DesktopLyricsColor = themes[next].id
	a.settings.save()
	if a.desktopLyrics != nil && a.desktopLyrics.win != nil {
		a.desktopLyrics.win.Invalidate()
	}
	a.redraw()
}

func (a *app) cycleDesktopLyricsFont() {
	fonts := lyricsFonts
	at := 0
	for i, f := range fonts {
		if f.id == a.settings.DesktopLyricsFont {
			at = i
			break
		}
	}
	next := (at + 1) % len(fonts)
	a.settings.DesktopLyricsFont = fonts[next].id
	a.settings.save()
	if a.desktopLyrics != nil && a.desktopLyrics.win != nil {
		a.desktopLyrics.win.Invalidate()
	}
	a.redraw()
}

func (a *app) cycleDesktopLyricsFontSize() {
	sizes := []float32{24, 28, 32, 38, 44}
	current := a.desktopLyricsFontSize()
	at := 2
	for i, sz := range sizes {
		if sz == current {
			at = i
			break
		}
	}
	next := (at + 1) % len(sizes)
	a.settings.DesktopLyricsFontSize = sizes[next]
	a.settings.save()
	if a.desktopLyrics != nil && a.desktopLyrics.win != nil {
		a.desktopLyrics.win.Invalidate()
	}
	a.redraw()
}

func (a *app) desktopLyricsFontSize() float32 {
	if a.settings.DesktopLyricsFontSize > 0 {
		return a.settings.DesktopLyricsFontSize
	}
	return 32
}
