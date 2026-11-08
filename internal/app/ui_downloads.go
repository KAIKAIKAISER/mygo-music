package app

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"mygo-music/internal/music"
)

func openInFileManager(path string) {
	if path == "" {
		return
	}
	switch runtime.GOOS {
	case "windows":
		cleanPath := filepath.Clean(path)
		if fi, err := os.Stat(cleanPath); err == nil && !fi.IsDir() {
			_ = exec.Command("explorer", "/select,"+cleanPath).Start()
		} else {
			_ = exec.Command("explorer", cleanPath).Start()
		}
	case "darwin":
		_ = exec.Command("open", "-R", path).Start()
	default:
		_ = exec.Command("xdg-open", filepath.Dir(path)).Start()
	}
}

func (a *app) chooseDownloadFolder() {
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
			Parent:            a.win,
			Title:             T("Change Download Folder"),
			Directory:         true,
			Multiple:          false,
			CreateDirectories: true,
		})
		if err == nil && len(paths) > 0 && paths[0] != "" {
			newDir := paths[0]
			a.post(func() {
				a.settings.DownloadDir = newDir
				a.settings.save()
				a.toast = Tf("Download directory changed to %s", newDir)
				a.redraw()
			})
		}
	}()
}

func (a *app) downloadSearchField(c *ui.Context, p palette) {
	field := ui.Row(c).Height(30).Width(200).Radius(8).AlignItems(ui.Center)
	if c.Focused(a.downloadFilterBox) {
		field.Background(p.bg).Shadow(0, 0, 0, 3, p.accent.Alpha(0.25)).Border(1, p.accent)
	} else {
		field.Background(p.hover).Border(1, p.hairline)
	}
	placeholder := T("Search downloaded songs")
	field.Children(func() {
		ui.TextInputBase(c.Key("dl-filter"), &a.downloadFilter).Bind(&a.downloadFilterBox).Grow(1).MinWidth(0).PaddingStart(29).PaddingEnd(9).
			Placeholder(placeholder).FontSize(12.5).Label(placeholder)
		ui.Icon(c, iconSearch).Size(13, 13).TextColor(p.muted).Absolute().Left(9).Top(8.5).IgnoreInput(true)
	})
}

func (a *app) downloadsPage(c *ui.Context) {
	mgr := a.downloadMgr
	if mgr == nil {
		return
	}
	p := a.pal

	completed := mgr.CompletedItems()
	activeTasks := mgr.ActiveAndQueuedItems()

	var totalBytes int64
	for _, it := range completed {
		totalBytes += it.TotalBytes
	}

	filter := strings.ToLower(strings.TrimSpace(a.downloadFilter))
	var filteredCompleted []*DownloadItem
	var filteredTracks []music.Track
	for _, it := range completed {
		if filter != "" {
			name := strings.ToLower(it.Track.Name)
			artist := strings.ToLower(it.Track.ArtistNames())
			if !strings.Contains(name, filter) && !strings.Contains(artist, filter) {
				continue
			}
		}
		filteredCompleted = append(filteredCompleted, it)
		filteredTracks = append(filteredTracks, it.AsTrack())
	}

	header := func(c *ui.Context) {
		ui.Column(c).Padding(0, 0, 24, 0).Gap(20).Children(func() {
			// Top Banner
			ui.Row(c).Gap(28).AlignItems(ui.End).Children(func() {
				ui.Box(c).Size(180, 180).Radius(14).Center().Shadow(0, 18, 40, -10, p.cardShade).TextColor(white).
					LinearGradient(ui.LinearGradient{Angle: 135, Oklab: true, From: ui.Oklch(0.65, 0.18, 240), To: ui.Oklch(0.55, 0.22, 290)}).
					Children(func() { ui.Icon(c, iconDownload).Size(72, 72) })

				ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Gap(8).Children(func() {
					ui.Text(c, T("Downloads")).FontSize(12).FontWeight(600).TextColor(p.accent)
					ui.Text(c, T("Download Management")).FontSize(28).FontWeight(800).LetterSpacing(-0.6)

					// Stats info
					statsText := Tf("%d songs", len(completed))
					if totalBytes > 0 {
						statsText = fmt.Sprintf("%s · %s %s", statsText, T("Used"), formatBytes(totalBytes))
					}
					ui.Text(c, statsText).FontSize(13).TextColor(p.muted)

					// Directory info & actions
					dlDir := a.downloadDir()
					ui.Row(c).Gap(10).AlignItems(ui.Center).Margin(6, 0, 0, 0).Children(func() {
						ui.Text(c, Tf("Storage location: %s", dlDir)).FontSize(12).TextColor(p.faint).SingleLine().Shrink(1).MinWidth(0)
						if pillButton(c.Key("change-dir"), nil, T("Change Folder"), p.hover.Over(p.bg), p.text).Height(28).PaddingX(12).Clicked() {
							a.chooseDownloadFolder()
						}
						if pillButton(c.Key("open-dir"), iconFolder, T("Open Folder"), p.hover.Over(p.bg), p.text).Height(28).PaddingX(12).Clicked() {
							openInFileManager(dlDir)
						}
					})
				})
			})

			// Controls & Tabs
			ui.Row(c).Gap(16).AlignItems(ui.Center).Children(func() {
				tab0Label := Tf("Downloaded (%d)", len(completed))
				tab1Label := Tf("Downloading (%d)", len(activeTasks))
				segmented(c, &a.downloadTab, tab0Label, tab1Label)

				if a.downloadTab == 0 {
					if len(filteredTracks) > 0 {
						a.playButtons(c, filteredTracks)
					}
					if len(completed) > 0 {
						if pillButton(c.Key("clear-completed"), iconTrash, T("Clear History"), p.hover.Over(p.bg), p.muted).Clicked() {
							a.downloadMgr.ClearCompleted()
						}
					}
					ui.Box(c).Grow(1)
					a.downloadSearchField(c, p)
				} else {
					if len(activeTasks) > 0 {
						hasFailed := false
						for _, it := range activeTasks {
							if it.Status == DownloadFailed {
								hasFailed = true
								break
							}
						}
						if hasFailed {
							if pillButton(c.Key("retry-all"), iconRefresh, T("Retry All"), p.hover.Over(p.bg), p.text).Clicked() {
								for _, it := range activeTasks {
									if it.Status == DownloadFailed {
										a.downloadMgr.Retry(it.ID)
									}
								}
							}
						}
						if pillButton(c.Key("cancel-all"), iconX, T("Cancel All"), p.hover.Over(p.bg), p.muted).Clicked() {
							for _, it := range activeTasks {
								if it.Status == DownloadActive || it.Status == DownloadQueued {
									a.downloadMgr.Cancel(it.ID)
								}
							}
						}
					}
					ui.Box(c).Grow(1)
				}
			})
		})
	}

	top, bottom := a.insets(c)
	a.underTop, a.underBottom = true, true

	if a.downloadTab == 0 {
		count := len(filteredCompleted)
		rows := count + 1
		if count == 0 {
			rows = 2
		}
		ui.List(c.Key("dl-tab-0"), &a.trackList, rows, func(i int) {
			if i == 0 {
				ui.Column(c).Children(func() {
					a.startMark(c)
					header(c)
				})
				return
			}
			if i == rows-1 {
				defer a.endMark(c)
			}
			if count == 0 {
				msg := T("No downloaded songs yet")
				if a.downloadFilter != "" {
					msg = T("No songs match the filter")
				}
				a.empty(c, msg)
				return
			}
			idx := i - 1
			a.downloadedRow(c, filteredCompleted, filteredTracks, idx)
		}).Grow(1).Padding(top+pageGap, pagePad-12, bottom+32, pagePad-12).ScrollbarInsets(top, 0, bottom, 0)
	} else {
		count := len(activeTasks)
		rows := count + 1
		if count == 0 {
			rows = 2
		}
		ui.List(c.Key("dl-tab-1"), &a.trackList, rows, func(i int) {
			if i == 0 {
				ui.Column(c).Children(func() {
					a.startMark(c)
					header(c)
				})
				return
			}
			if i == rows-1 {
				defer a.endMark(c)
			}
			if count == 0 {
				a.empty(c, T("No downloading tasks"))
				return
			}
			idx := i - 1
			a.downloadingRow(c, activeTasks[idx])
		}).Grow(1).Padding(top+pageGap, pagePad-12, bottom+32, pagePad-12).ScrollbarInsets(top, 0, bottom, 0)
	}
}

func (a *app) downloadedRow(c *ui.Context, items []*DownloadItem, tracks []music.Track, i int) {
	p := a.pal
	it := items[i]
	tr := &tracks[i]
	playing := sameTrack(a.pl.Current(), tr)
	row := ui.ButtonBase(c.Key(it.ID)).Height(54).PaddingX(12).Gap(14).Radius(10).AlignItems(ui.Center).FocusRing(false).Label(tr.Name).
		ContextMenu(a.songItems(tr, ""))
	hovered := row.Hovered()
	bg := ui.Transparent
	switch {
	case playing:
		bg = p.accent.Alpha(0.09)
	case hovered:
		bg = p.hover
	}
	row.Background(row.AnimateColor("bg", bg, ui.Animation{Duration: 100 * time.Millisecond}))
	row.Children(func() {
		// Index or play indicator
		ui.Box(c).Width(26).Center().Children(func() {
			switch {
			case playing:
				a.nowPlayingBars(c)
			case hovered:
				ui.Icon(c, iconPlay).Size(13, 13).TextColor(p.text)
			default:
				ui.Textf(c, "%d", i+1).FontSize(13).FontFeatures("tnum").TextColor(p.faint)
			}
		})
		// Cover
		a.cover(c, tr.Album.ImageURL, 96, 38, 6)
		// Title and Artist
		ui.Column(c).Grow(3).Basis(0).MinWidth(0).Gap(3).Children(func() {
			nameColor := p.text
			if playing {
				nameColor = p.accent
			}
			ui.Text(c, tr.Name).FontSize(14).FontWeight(500).TextColor(nameColor).SingleLine()
			a.artistLinks(c, tr, p.muted, p.text).FontSize(12.5).SingleLine().Shrink(1).MinWidth(0)
		})
		// File format & size badge
		ext := it.Ext
		if ext == "" {
			ext = strings.ToUpper(strings.TrimPrefix(filepath.Ext(it.Path), "."))
		} else {
			ext = strings.ToUpper(ext)
		}
		badgeText := ext
		if sizeStr := formatBytes(it.TotalBytes); sizeStr != "" && it.TotalBytes > 0 {
			badgeText = ext + " · " + sizeStr
		}
		ui.Box(c).Padding(3, 8).Radius(6).Background(p.hover).Children(func() {
			ui.Text(c, badgeText).FontSize(11).FontWeight(600).TextColor(p.faint).SingleLine()
		})
		// Action buttons (Open Folder, Delete)
		ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
			if hovered {
				openBtn := iconButton(c.Key("open-folder-"+it.ID), iconFolder, 14, p.muted, T("Open in Folder"))
				if openBtn.Clicked() {
					openInFileManager(it.Path)
				}
				delBtn := iconButton(c.Key("del-download-"+it.ID), iconTrash, 14, p.muted, T("Delete Download"))
				if delBtn.Clicked() {
					a.downloadMgr.Delete(it.ID, true)
				}
			}
		})
		// Duration
		ui.Text(c, clock(tr.Length())).FontSize(12.5).FontFeatures("tnum").TextColor(p.faint).Width(44).TextAlign(ui.End)
	})
	if row.Clicked() {
		if sameTrack(a.pl.Current(), tr) {
			if !a.pl.Playing() && !a.pl.Loading() {
				a.pl.TogglePlay()
			}
			return
		}
		a.pl.PlayTrack(tracks, i)
	}
}

func (a *app) downloadingRow(c *ui.Context, it *DownloadItem) {
	p := a.pal
	card := ui.Column(c.Key(it.ID)).Padding(12, 14).Radius(10).Gap(8).Background(p.hover.Alpha(0.6)).Margin(0, 0, 6, 0)
	card.Children(func() {
		ui.Row(c).AlignItems(ui.Center).Gap(12).Children(func() {
			// Cover
			a.cover(c, it.Track.Album.ImageURL, 96, 38, 6)
			// Info
			ui.Column(c).Grow(1).MinWidth(0).Gap(3).Children(func() {
				ui.Text(c, it.Track.Name).FontSize(14).FontWeight(500).TextColor(p.text).SingleLine()
				ui.Text(c, it.Track.ArtistNames()).FontSize(12.5).TextColor(p.muted).SingleLine()
			})
			// Status text
			ui.Column(c).AlignItems(ui.End).Gap(2).Children(func() {
				switch it.Status {
				case DownloadActive:
					pct := int(it.Progress * 100)
					if pct > 100 {
						pct = 100
					}
					statusStr := fmt.Sprintf("%d%%", pct)
					if it.TotalBytes > 0 {
						statusStr = fmt.Sprintf("%s / %s (%d%%)", formatBytes(it.BytesDone), formatBytes(it.TotalBytes), pct)
					}
					ui.Text(c, statusStr).FontSize(12.5).FontFeatures("tnum").TextColor(p.accent).SingleLine()
				case DownloadQueued:
					ui.Text(c, T("Queued")).FontSize(12.5).TextColor(p.muted).SingleLine()
				case DownloadFailed:
					errText := T("Download Failed")
					if it.Error != "" {
						errText += ": " + it.Error
					}
					ui.Text(c, errText).FontSize(12).TextColor(ui.Hex("#ff4d4f")).SingleLine()
				case DownloadCancelled:
					ui.Text(c, T("Cancelled")).FontSize(12.5).TextColor(p.muted).SingleLine()
				}
			})
			// Controls
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				if it.Status == DownloadActive || it.Status == DownloadQueued {
					cancelBtn := iconButton(c.Key("cancel-"+it.ID), iconX, 15, p.muted, T("Cancel"))
					if cancelBtn.Clicked() {
						a.downloadMgr.Cancel(it.ID)
					}
				} else if it.Status == DownloadFailed || it.Status == DownloadCancelled {
					retryBtn := iconButton(c.Key("retry-"+it.ID), iconRefresh, 15, p.muted, T("Retry"))
					if retryBtn.Clicked() {
						a.downloadMgr.Retry(it.ID)
					}
					delBtn := iconButton(c.Key("del-"+it.ID), iconTrash, 15, p.muted, T("Delete"))
					if delBtn.Clicked() {
						a.downloadMgr.Delete(it.ID, false)
					}
				}
			})
		})
		// Progress bar (if active)
		if it.Status == DownloadActive {
			ui.Box(c).Height(4).Radius(2).Background(p.hairline).Draw(func(painter *ui.Painter, r ui.Rect) {
				prog := it.Progress
				if prog < 0 {
					prog = 0
				} else if prog > 1 {
					prog = 1
				}
				if prog > 0 {
					barRect := ui.Rect{
						X: r.X,
						Y: r.Y,
						W: r.W * prog,
						H: r.H,
					}
					painter.Fill(barRect, p.accent, 2)
				}
			})
		}
	})
}
