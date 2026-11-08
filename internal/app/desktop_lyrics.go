package app

import (
	"fmt"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"mygo-music/internal/lyrics"
)

type lyricsTheme struct {
	id    string
	name  string
	color ui.Color
}

var lyricsThemes = []lyricsTheme{
	{"gold", "Sunset Gold", ui.RGB(255, 184, 0)},     // NetEase Gold #FFB800
	{"cyan", "Aurora Cyan", ui.RGB(0, 226, 255)},     // Aurora Cyan #00E2FF
	{"red", "Vibrant Red", ui.RGB(255, 75, 75)},       // Vibrant Red #FF4B4B
	{"green", "Fresh Green", ui.RGB(38, 222, 129)},    // Fresh Green #26DE81
	{"pink", "Sakura Pink", ui.RGB(255, 107, 129)},    // Sakura Pink #FF6B81
	{"white", "Pure White", ui.RGB(255, 255, 255)},    // Pure White #FFFFFF
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

	w, h    float32
	screenX float32
	screenY float32
	locked  bool

	mouseInside bool
	overUnlock  bool
}

func newDesktopLyrics(a *app) *desktopLyrics {
	d := &desktopLyrics{a: a}

	disp := mygo.Screen.PrimaryDisplay()
	area := disp.WorkArea
	if area.Width == 0 || area.Height == 0 {
		area = disp.Bounds
	}
	defaultW := 1024
	if area.Width > 0 && area.Width-40 < defaultW {
		defaultW = max(600, area.Width-40)
	}
	d.w = float32(defaultW)
	d.h = 140

	if area.Width > 0 && area.Height > 0 {
		d.screenX = float32(area.X + (area.Width-int(d.w))/2)
		d.screenY = float32(area.Y + area.Height - int(d.h) - 70)
	}

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
		X:               int(d.screenX),
		Y:               int(d.screenY),
		Width:           int(d.w),
		Height:          int(d.h),
		Content:         ui.View(d.view),
	})

	removeWindowBorder(d.win.NativeHandle())
	d.adjustWithinScreen()

	d.win.OnShow(func() {
		removeWindowBorder(d.win.NativeHandle())
		d.adjustWithinScreen()
	})

	d.win.OnMove(func() {
		d.adjustWithinScreen()
	})

	d.win.OnResize(func() {
		d.adjustWithinScreen()
	})

	d.win.OnClose(func(e *mygo.CloseEvent) {
		if !a.quitting {
			e.PreventDefault()
			a.setDesktopLyrics(false)
		}
	})

	if a.settings.DesktopLyrics {
		removeWindowBorder(d.win.NativeHandle())
		d.adjustWithinScreen()
		d.win.ShowInactive()
		d.applyLock(a.settings.DesktopLyricsLocked)
	}

	return d
}

func (d *desktopLyrics) show() {
	if d.win != nil {
		removeWindowBorder(d.win.NativeHandle())
		d.adjustWithinScreen()
		d.win.ShowInactive()
		d.applyLock(d.a.settings.DesktopLyricsLocked)
	}
}

func (d *desktopLyrics) hide() {
	if d.win != nil {
		d.win.Hide()
	}
}

func (d *desktopLyrics) adjustWithinScreen() {
	if d.win == nil || !d.win.IsVisible() {
		return
	}
	b := d.win.Bounds()
	disp := mygo.Screen.DisplayMatching(b)
	area := disp.WorkArea
	if area.Width == 0 || area.Height == 0 {
		area = disp.Bounds
	}
	if area.Width == 0 || area.Height == 0 {
		return
	}

	const margin = 10
	changed := false

	if b.Width > area.Width-margin*2 {
		b.Width = area.Width - margin*2
		changed = true
	}
	if b.Height > area.Height-margin*2 {
		b.Height = area.Height - margin*2
		changed = true
	}

	if b.X+b.Width > area.X+area.Width-margin {
		b.X = area.X + area.Width - margin - b.Width
		changed = true
	}
	if b.X < area.X+margin {
		b.X = area.X + margin
		changed = true
	}
	if b.Y+b.Height > area.Y+area.Height-margin {
		b.Y = area.Y + area.Height - margin - b.Height
		changed = true
	}
	if b.Y < area.Y+margin {
		b.Y = area.Y + margin
		changed = true
	}

	if changed {
		d.win.SetBounds(b)
	}
}

func (d *desktopLyrics) applyLock(locked bool) {
	d.locked = locked
	d.mouseInside = false
	d.overUnlock = false
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

	// Ensure window stays within screen bounds at all times
	d.adjustWithinScreen()

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

	// Line 2: Next upcoming line or translation
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

	// Schedule redraw while playing; if paused, periodically check screen edges and hover
	if pl.Playing() {
		if cur >= 0 && cur < len(lines) && len(lines[cur].Words) > 0 {
			// Syllable / karaoke precision (25 fps).
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
		c.After(300 * time.Millisecond)
	}

	locked := a.settings.DesktopLyricsLocked
	fontSize := a.desktopLyricsFontSize()
	fontFamily := getFontFamily(a.settings.DesktopLyricsFont)
	activeColor := getLyricsColor(a.settings.DesktopLyricsColor)
	secondaryColor := ui.RGBA(255, 255, 255, 0.78)

	// Window bounds and display work area
	b := d.win.Bounds()
	curWinW := float32(b.Width)
	if curWinW <= 0 {
		curWinW = d.w
	}
	disp := mygo.Screen.DisplayMatching(b)
	area := disp.WorkArea
	if area.Width == 0 || area.Height == 0 {
		area = disp.Bounds
	}
	maxWinW := float32(area.Width - 20)
	if maxWinW < 600 {
		maxWinW = 600
	}

	// Measure required text width at base font size
	w1 := measureText(c, line1Text, fontFamily, fontSize, 800)
	baseSize2 := max(16, fontSize*0.72)
	w2 := float32(0)
	if line2Text != "" {
		w2 = measureText(c, line2Text, fontFamily, baseSize2, 600)
	}

	// Dynamic width expansion: expand window width if lyrics need more room
	neededW := max(w1, w2) + 64
	if neededW > curWinW && curWinW < maxWinW {
		targetW := min(maxWinW, neededW)
		if targetW > curWinW+16 {
			d.w = targetW
			b.Width = int(targetW)
			if b.X+b.Width > area.X+area.Width-10 {
				b.X = area.X + area.Width - 10 - b.Width
			}
			if b.X < area.X+10 {
				b.X = area.X + 10
			}
			d.win.SetBounds(b)
			b = d.win.Bounds()
			curWinW = float32(b.Width)
		}
	}

	// Usable width for text inside wrap (with margin padding)
	usableW := curWinW - 48
	if usableW < 400 {
		usableW = 400
	}

	// Adaptive font size scaling for Line 1: downscale smoothly if it exceeds usable width
	line1FontSize := fontSize
	if w1 > usableW && w1 > 0 {
		scale1 := usableW / w1
		line1FontSize = max(18.0, fontSize*scale1)
	}

	// Adaptive font size scaling for Line 2
	line2FontSize := baseSize2
	if w2 > usableW && w2 > 0 {
		scale2 := usableW / w2
		line2FontSize = max(13.0, baseSize2*scale2)
	}

	if locked {
		c.After(50 * time.Millisecond)
		if d.win != nil && d.win.IsVisible() {
			pt := mygo.Screen.CursorScreenPoint()
			inside := pt.X >= b.X && pt.X < b.X+b.Width && pt.Y >= b.Y && pt.Y < b.Y+b.Height

			btnW, btnH := 180, 36
			btnX := b.X + (b.Width-btnW)/2
			btnY := b.Y + 2
			overBtn := inside && pt.X >= btnX && pt.X <= btnX+btnW && pt.Y >= btnY && pt.Y <= btnY+btnH

			if inside != d.mouseInside || overBtn != d.overUnlock {
				d.mouseInside = inside
				d.overUnlock = overBtn
				d.win.SetIgnoreMouseEvents(!overBtn)
				d.win.Invalidate()
			}
		}
	} else {
		d.mouseInside = false
		d.overUnlock = false
	}

	wrap := ui.Box(c.Key("desktop-wrap")).Fill().Padding(4, 16).Background(ui.Transparent)
	if !locked {
		wrap.DragWindow()
	}

	wrap.Children(func() {
		ui.Column(c).Fill().Justify(ui.SpaceBetween).Children(func() {
			// Top row: hover toolbar when unlocked, or unlock button when locked and mouse is over lyrics
			if !locked {
				if wrap.Hovered() {
					d.toolbar(c, activeColor)
				} else {
					ui.Box(c).Height(28)
				}
			} else {
				if d.mouseInside {
					ui.Row(c.Key("unlock-wrap")).Height(28).Justify(ui.Center).Children(func() {
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

			// Lyrics rows container
			ui.Column(c).Grow(1).Justify(ui.Center).Gap(4).Children(func() {
				ui.Row(c.Key("line1-row")).Justify(ui.Center).AlignItems(ui.Center).Children(func() {
					if len(currentWords) > 0 && cur >= 0 && cur < len(lines) {
						// Karaoke word-by-word lighting up!
						rt := ui.RichText(c.Key("rt1")).FontSize(line1FontSize).FontWeight(800).
							LineHeight(1.15).LetterSpacing(-0.2).
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
						el := ui.Text(c.Key("txt1"), line1Text).FontSize(line1FontSize).FontWeight(800).
							LineHeight(1.15).LetterSpacing(-0.2).
							TextColor(activeColor).
							TextStroke(2.2, ui.RGBA(0, 0, 0, 0.8)).
							TextShadow(0, 2, 8, ui.RGBA(0, 0, 0, 0.95)).
							SingleLine()
						if fontFamily != "" {
							el.Font(fontFamily)
						}
					}
				})

				if line2Text != "" {
					ui.Row(c.Key("line2-row")).Justify(ui.Center).AlignItems(ui.Center).Children(func() {
						el := ui.Text(c.Key("txt2"), line2Text).FontSize(line2FontSize).FontWeight(600).
							LineHeight(1.2).LetterSpacing(-0.1).
							TextColor(secondaryColor).
							TextStroke(1.6, ui.RGBA(0, 0, 0, 0.75)).
							TextShadow(0, 2, 6, ui.RGBA(0, 0, 0, 0.85)).
							SingleLine()
						if fontFamily != "" {
							el.Font(fontFamily)
						}
					})
				}
			})

			// Bottom padding spacer
			ui.Box(c).Height(4)
		})
	})
}

func measureText(c *ui.Context, text, font string, size float32, weight int) float32 {
	if text == "" || size <= 0 {
		return 0
	}
	w, _ := c.MeasureText(0, ui.Span{
		Text:   text,
		Font:   font,
		Size:   size,
		Weight: weight,
	})
	if w <= 0 {
		return estimateTextWidth(text, size)
	}
	return w
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

func (d *desktopLyrics) toolbar(c *ui.Context, activeColor ui.Color) {
	a := d.a
	pl := a.pl

	ui.Row(c.Key("tb-wrap")).Justify(ui.Center).Children(func() {
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
