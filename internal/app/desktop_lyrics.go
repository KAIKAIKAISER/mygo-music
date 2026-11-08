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
}

func newDesktopLyrics(a *app) *desktopLyrics {
	d := &desktopLyrics{a: a}
	d.w, d.h = 880, 140

	disp := mygo.Screen.PrimaryDisplay()
	area := disp.WorkArea
	if area.Width == 0 || area.Height == 0 {
		area = disp.Bounds
	}
	if area.Width > 0 && area.Height > 0 {
		d.screenX = float32(area.X + (area.Width-int(d.w))/2)
		d.screenY = float32(area.Y + area.Height - int(d.h) - 70)
	}

	d.win = mygo.NewWindow(mygo.WindowOptions{
		Title:         Tf("%s Desktop Lyrics", mygo.App.Name()),
		Panel:         true,
		Hidden:        true,
		Frameless:     true,
		Transparent:   true,
		AlwaysOnTop:   true,
		DisableShadow: true,
		DisableResize: true,
		SkipTaskbar:   true,
		StateKey:      "desktop-lyrics",
		X:             int(d.screenX),
		Y:             int(d.screenY),
		Width:         int(d.w),
		Height:        int(d.h),
		Content:       ui.View(d.view),
	})

	d.win.OnClose(func(e *mygo.CloseEvent) {
		if !a.quitting {
			e.PreventDefault()
			a.setDesktopLyrics(false)
		}
	})

	if a.settings.DesktopLyrics {
		d.win.ShowInactive()
		d.applyLock(a.settings.DesktopLyricsLocked)
	}

	return d
}

func (d *desktopLyrics) show() {
	if d.win != nil {
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
	a := d.a
	pl := a.pl
	tr := pl.Current()
	lines, _ := pl.Lyrics()
	pos := pl.Position()
	cur := -1
	if len(lines) > 0 {
		cur = lyrics.Index(lines, pos)
	}

	// Schedule redraw while playing.
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
	}

	locked := a.settings.DesktopLyricsLocked
	fontSize := a.desktopLyricsFontSize()
	fontFamily := getFontFamily(a.settings.DesktopLyricsFont)
	activeColor := getLyricsColor(a.settings.DesktopLyricsColor)
	secondaryColor := ui.RGBA(255, 255, 255, 0.78)

	wrap := ui.Box(c.Key("desktop-wrap")).Fill().Padding(6, 16)
	if !locked {
		wrap.DragWindow()
	}

	hovered := wrap.Hovered() && !locked
	if hovered {
		wrap.Background(ui.RGBA(18, 18, 22, 0.65)).Radius(16).
			Border(1, ui.RGBA(255, 255, 255, 0.15)).
			Shadow(0, 10, 32, -4, ui.RGBA(0, 0, 0, 0.45))
	}

	wrap.Children(func() {
		ui.Column(c).Fill().Justify(ui.SpaceBetween).Children(func() {
			// Top row: hover toolbar (only when hovered and not locked)
			if hovered {
				d.toolbar(c, activeColor)
			} else {
				ui.Box(c).Height(6)
			}

			// Lyrics rows container
			ui.Column(c).Grow(1).Justify(ui.Center).Gap(4).Children(func() {
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

				ui.Row(c.Key("line1-row")).Justify(ui.Center).AlignItems(ui.Center).Children(func() {
					if len(currentWords) > 0 && cur >= 0 && cur < len(lines) {
						// Karaoke word-by-word lighting up!
						rt := ui.RichText(c.Key("rt1")).FontSize(fontSize).FontWeight(800).
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
						el := ui.Text(c.Key("txt1"), line1Text).FontSize(fontSize).FontWeight(800).
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

				if line2Text != "" {
					line2Size := max(16, fontSize*0.72)
					ui.Row(c.Key("line2-row")).Justify(ui.Center).AlignItems(ui.Center).Children(func() {
						el := ui.Text(c.Key("txt2"), line2Text).FontSize(line2Size).FontWeight(600).
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
