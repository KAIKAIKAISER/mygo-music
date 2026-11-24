package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"mygo-music/internal/lyrics"
	"mygo-music/internal/music"
	"mygo-music/internal/netease"
)

// savedLyric stores a listener's manually chosen lyrics for a specific track
// so replays use the chosen lyrics immediately without hitting auto-match again.
type savedLyric struct {
	SongID      int64  `json:"songId"`
	Title       string `json:"title"`
	Artist      string `json:"artist"`
	LyricsLRC   string `json:"lrc"`
	Translation string `json:"trans,omitempty"`
	CoverURL    string `json:"coverUrl,omitempty"`
}

type lyricsStore struct {
	mu    sync.RWMutex
	items map[string]savedLyric
	path  string
}

func trackLyricKey(source, id string) string {
	return source + "\x00" + id
}

func loadLyricsStore() *lyricsStore {
	s := &lyricsStore{
		items: make(map[string]savedLyric),
	}
	if dir, err := mygo.App.Path(mygo.PathUserData); err == nil && dir != "" {
		s.path = filepath.Join(dir, "lyrics-choices.json")
		if b, err := os.ReadFile(s.path); err == nil {
			_ = json.Unmarshal(b, &s.items)
			if s.items == nil {
				s.items = make(map[string]savedLyric)
			}
		}
	}
	return s
}

func (s *lyricsStore) get(source, id string) *music.Lyrics {
	if s == nil {
		return nil
	}
	s.mu.RLock()
	item, ok := s.items[trackLyricKey(source, id)]
	s.mu.RUnlock()
	if !ok || item.LyricsLRC == "" {
		return nil
	}
	return &music.Lyrics{
		LRC:         item.LyricsLRC,
		Translation: item.Translation,
	}
}

func (s *lyricsStore) has(source, id string) bool {
	if s == nil {
		return false
	}
	s.mu.RLock()
	_, ok := s.items[trackLyricKey(source, id)]
	s.mu.RUnlock()
	return ok
}

func (s *lyricsStore) set(source, id string, item savedLyric) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.items[trackLyricKey(source, id)] = item
	s.saveLocked()
	s.mu.Unlock()
}

func (s *lyricsStore) remove(source, id string) {
	if s == nil {
		return
	}
	s.mu.Lock()
	delete(s.items, trackLyricKey(source, id))
	s.saveLocked()
	s.mu.Unlock()
}

func (s *lyricsStore) saveLocked() {
	if s.path == "" {
		return
	}
	b, err := json.MarshalIndent(s.items, "", "  ")
	if err != nil {
		return
	}
	_ = os.MkdirAll(filepath.Dir(s.path), 0o700)
	_ = os.WriteFile(s.path, b, 0o600)
}

type lyricsPicker struct {
	open        bool
	track       *music.Track
	query       string
	searching   bool
	searched    bool
	searchGen   int
	results     []netease.MatchResult
	selected    int
	previewGen  int
	previewBusy bool
	previewLRC  string
	previewTr   string
	previewRows []lyrics.Line
	applyOnLoad bool
	scroll      ui.ScrollState
	previewSc   ui.ScrollState
}

var (
	neteaseSearchLyrics = netease.Search
	neteaseFetchLyrics  = netease.FetchLyrics
)

// openLyricsPicker opens the manual lyrics selection dialog for tr,
// bringing the main window forward if triggered from desktop lyrics.
func (a *app) openLyricsPicker(tr *music.Track) {
	if tr == nil {
		return
	}
	a.showMain()
	copyTr := *tr
	a.lyricsPick = lyricsPicker{
		open:     true,
		track:    &copyTr,
		query:    netease.SuggestQuery(tr.ArtistNames(), tr.Name),
		selected: -1,
	}
	a.searchLyricsPicker()
	a.redraw()
}

func (a *app) searchLyricsPicker() {
	d := &a.lyricsPick
	q := strings.TrimSpace(d.query)
	if q == "" {
		return
	}
	d.searchGen++
	gen := d.searchGen
	d.searching = true
	d.searched = false
	d.results = nil
	d.selected = -1
	d.previewLRC = ""
	d.previewTr = ""
	d.previewRows = nil
	d.previewBusy = false
	d.applyOnLoad = false
	d.scroll = ui.ScrollState{}

	go func() {
		res := neteaseSearchLyrics(q, 12)
		a.post(func() {
			if gen != a.lyricsPick.searchGen || !a.lyricsPick.open {
				return
			}
			a.lyricsPick.searching = false
			a.lyricsPick.searched = true
			a.lyricsPick.results = res
			if len(res) > 0 {
				a.previewPickedLyric(0)
			}
		})
	}()
}

func (a *app) previewPickedLyric(idx int) {
	d := &a.lyricsPick
	if idx < 0 || idx >= len(d.results) {
		return
	}
	d.selected = idx
	d.previewGen++
	gen := d.previewGen
	d.previewBusy = true
	d.previewLRC = ""
	d.previewTr = ""
	d.previewRows = nil
	d.applyOnLoad = false
	d.previewSc = ui.ScrollState{}
	songID := d.results[idx].SongID

	go func() {
		lrc, trans := neteaseFetchLyrics(songID)
		var rows []lyrics.Line
		if lrc != "" {
			rows = lyrics.FromSource(&music.Lyrics{
				LRC:         lrc,
				Translation: trans,
			})
		}
		a.post(func() {
			if gen != a.lyricsPick.previewGen || !a.lyricsPick.open {
				return
			}
			a.lyricsPick.previewBusy = false
			a.lyricsPick.previewLRC = lrc
			a.lyricsPick.previewTr = trans
			a.lyricsPick.previewRows = rows
			if a.lyricsPick.applyOnLoad {
				a.lyricsPick.applyOnLoad = false
				a.applyPickedLyric()
			}
		})
	}()
}

func (a *app) applyPickedLyric() {
	d := &a.lyricsPick
	if d.track == nil || d.selected < 0 || d.selected >= len(d.results) {
		return
	}
	if d.previewBusy {
		d.applyOnLoad = true
		return
	}
	if len(d.previewRows) == 0 && d.previewLRC == "" {
		a.toast = T("This track has no timed lyrics")
		return
	}

	cand := d.results[d.selected]
	lines := d.previewRows
	if len(lines) == 0 {
		lines = lyrics.FromSource(&music.Lyrics{
			LRC:         d.previewLRC,
			Translation: d.previewTr,
		})
	}
	if len(lines) == 0 {
		a.toast = T("This track has no timed lyrics")
		return
	}

	if a.lyricsChoices != nil {
		a.lyricsChoices.set(d.track.Source, d.track.ID, savedLyric{
			SongID:      cand.SongID,
			Title:       cand.Title,
			Artist:      cand.Artist,
			LyricsLRC:   d.previewLRC,
			Translation: d.previewTr,
			CoverURL:    cand.CoverURL,
		})
	}

	if a.pl != nil {
		if cur := a.pl.Current(); cur != nil && cur.Source == d.track.Source && cur.ID == d.track.ID {
			if cur.Album.ImageURL == "" && cand.CoverURL != "" {
				cur.Album.ImageURL = cand.CoverURL
			}
			a.pl.ApplyLyrics(lines)
		}
	}
	if a.desktopLyrics != nil {
		a.desktopLyrics.redraw()
	}
	a.toast = Tf("Switched lyrics to “%s”", cand.Title)
	d.open = false
}

func (a *app) resetPickedLyric() {
	d := &a.lyricsPick
	if d.track == nil {
		return
	}
	if a.lyricsChoices != nil {
		a.lyricsChoices.remove(d.track.Source, d.track.ID)
	}
	if a.pl != nil {
		if cur := a.pl.Current(); cur != nil && cur.Source == d.track.Source && cur.ID == d.track.ID {
			a.pl.ReloadLyrics()
		}
	}
	if a.desktopLyrics != nil {
		a.desktopLyrics.redraw()
	}
	a.toast = T("Restored default lyrics matching")
	d.open = false
}

func (a *app) lyricsPickerDialog(c *ui.Context) {
	p := a.pal
	d := &a.lyricsPick
	ui.DialogBase(c.Key("lyrics-picker"), &d.open, func(backdrop, panel ui.Element) {
		backdrop.Background(ui.RGBA(0, 0, 0, 0.42))
		panel.Width(540).Padding(22).Radius(18).Gap(14).
			Background(p.bg).Border(1, p.hairline).Shadow(0, 24, 60, -12, ui.RGBA(0, 0, 0, 0.45)).
			Transition(ui.ElementTransition{Animation: &ui.Snappy, Enter: &ui.Motion{Scale: 0.94}, Exit: &ui.Motion{Scale: 0.96}})

		// Header
		ui.Row(c).AlignItems(ui.Center).Gap(10).Children(func() {
			ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
				ui.Text(c, T("Choose Lyrics")).FontSize(17).FontWeight(700).TextColor(p.text)
				if d.track != nil {
					sub := d.track.Name
					if art := d.track.ArtistNames(); art != "" {
						sub += " — " + art
					}
					ui.Text(c, sub).FontSize(12).TextColor(p.muted).SingleLine()
				}
			})
			if d.track != nil && a.lyricsChoices.has(d.track.Source, d.track.ID) {
				if pillButton(c.Key("reset-lyrics"), iconRefresh, T("Reset to Default"), p.hover.Over(p.bg), p.text).Height(28).PaddingX(11).Clicked() {
					a.resetPickedLyric()
				}
			}
		})

		// Search input row
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			in := ui.TextInput(c.Key("lyrics-query"), &d.query).
				Grow(1).MinWidth(0).
				Placeholder(T("Search NetEase Cloud Music by song or artist…")).
				Label(T("Choose Lyrics")).
				FontSize(13.5).
				AutoFocus()
			if in.Submitted() {
				a.searchLyricsPicker()
			}
			searchLabel := T("Search")
			if d.searching {
				searchLabel = T("Searching…")
			}
			sb := pillButton(c.Key("search-lyrics"), iconSearch, searchLabel, p.accent, p.onAccent).Height(34).PaddingX(14)
			if strings.TrimSpace(d.query) == "" || d.searching {
				sb.Disabled(true).Opacity(0.55)
			}
			if sb.Clicked() {
				a.searchLyricsPicker()
			}
		})

		// Candidate results list
		listBox := ui.Box(c).Height(220).Radius(12).Border(1, p.hairline).Background(p.sidebar).Padding(6).Children(func() {
			if d.searching {
				ui.Column(c).Fill().Center().Children(func() {
					ui.Text(c, T("Searching…")).FontSize(13).TextColor(p.muted)
				})
				return
			}
			if len(d.results) == 0 {
				ui.Column(c).Fill().Center().Padding(0, 16).Children(func() {
					msg := T("No matching lyrics found. Try editing the keywords.")
					if !d.searched {
						msg = T("Search NetEase Cloud Music by song or artist…")
					}
					ui.Text(c, msg).FontSize(13).TextColor(p.muted).TextAlign(ui.Center)
				})
				return
			}
			ui.Scroll(c.Key("cand-scroll")).Fill().TrackScroll(&d.scroll).Children(func() {
				ui.Column(c).Gap(3).Children(func() {
					for i, item := range d.results {
						selected := i == d.selected
						row := ui.ButtonBase(c.Key(i)).Padding(8, 10).Radius(9).AlignItems(ui.Center).FocusRing(false).Label(item.Title)
						bg := ui.Transparent
						if selected {
							bg = p.accent.Alpha(0.15)
							row.Border(1, p.accent.Alpha(0.5))
						} else if row.Hovered() {
							bg = p.hover.Over(p.sidebar)
						}
						row.Background(bg)
						if row.Clicked() {
							if selected && !d.previewBusy && len(d.previewRows) > 0 {
								a.applyPickedLyric()
							} else {
								a.previewPickedLyric(i)
							}
						}
						row.Children(func() {
							ui.Row(c).Fill().AlignItems(ui.Center).Gap(10).Children(func() {
								ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
									titleCol := p.text
									if selected {
										titleCol = p.accent
									}
									ui.Text(c, item.Title).FontSize(13).FontWeight(600).TextColor(titleCol).SingleLine()
									meta := item.Artist
									if item.Album != "" {
										if meta != "" {
											meta += " · "
										}
										meta += item.Album
									}
									ui.Text(c, meta).FontSize(11.5).TextColor(p.muted).SingleLine()
								})
								if item.Duration > 0 {
									ui.Text(c, clock(item.Duration)).FontSize(11.5).FontFeatures("tnum").TextColor(p.faint)
								}
							})
						})
					}
				})
			})
		})
		_ = listBox

		// Lyrics preview box
		ui.Box(c).Height(112).Radius(12).Border(1, p.hairline).Background(p.sidebar).Padding(10, 12).Children(func() {
			if d.selected < 0 {
				ui.Column(c).Fill().Center().Children(func() {
					ui.Text(c, T("Loading lyrics preview…")).FontSize(12).TextColor(p.faint)
				})
				return
			}
			if d.previewBusy {
				ui.Column(c).Fill().Center().Children(func() {
					ui.Text(c, T("Loading lyrics preview…")).FontSize(12.5).TextColor(p.muted)
				})
				return
			}
			if len(d.previewRows) == 0 {
				ui.Column(c).Fill().Center().Children(func() {
					ui.Text(c, T("This track has no timed lyrics")).FontSize(12.5).TextColor(p.muted)
				})
				return
			}
			ui.Scroll(c.Key("preview-scroll")).Fill().TrackScroll(&d.previewSc).Children(func() {
				ui.Column(c).Gap(4).Children(func() {
					for i, l := range d.previewRows {
						if i >= 24 {
							break
						}
						ui.Text(c, l.Text).FontSize(12.5).FontWeight(600).TextColor(p.text).SingleLine()
						if l.Translation != "" {
							ui.Text(c, l.Translation).FontSize(11.5).TextColor(p.muted).SingleLine()
						}
					}
				})
			})
		})

		// Footer actions
		ui.Row(c).AlignItems(ui.Center).Gap(10).Children(func() {
			hint := ""
			if d.selected >= 0 && !d.previewBusy && len(d.previewRows) > 0 {
				if d.previewTr != "" {
					hint = T("Includes translation")
				} else {
					hint = T("Original lyrics only")
				}
			}
			ui.Text(c, hint).FontSize(12).TextColor(p.muted).Grow(1).MinWidth(0).SingleLine()

			if pillButton(c.Key("cancel"), nil, T("Cancel"), p.hover.Over(p.bg), p.text).Clicked() {
				d.open = false
			}
			apply := pillButton(c.Key("apply"), nil, T("Apply Lyrics"), p.accent, p.onAccent)
			canApply := d.selected >= 0 && (d.previewBusy || len(d.previewRows) > 0)
			if !canApply {
				apply.Disabled(true).Opacity(0.5)
			}
			if apply.Clicked() {
				a.applyPickedLyric()
			}
		})
	})
}
