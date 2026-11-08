package app

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo"

	"mygo-music/internal/music"
)

type DownloadStatus int

const (
	DownloadQueued DownloadStatus = iota
	DownloadActive
	DownloadDone
	DownloadFailed
	DownloadCancelled
)

func (s DownloadStatus) String() string {
	switch s {
	case DownloadQueued:
		return "Queued"
	case DownloadActive:
		return "Downloading"
	case DownloadDone:
		return "Completed"
	case DownloadFailed:
		return "Failed"
	case DownloadCancelled:
		return "Cancelled"
	}
	return "Unknown"
}

type DownloadItem struct {
	ID          string         `json:"id"`
	Track       music.Track    `json:"track"`
	Path        string         `json:"path"`
	Status      DownloadStatus `json:"status"`
	Progress    float32        `json:"progress"`
	BytesDone   int64          `json:"bytesDone"`
	TotalBytes  int64          `json:"totalBytes"`
	Error       string         `json:"error,omitempty"`
	CreatedAt   time.Time      `json:"createdAt"`
	CompletedAt time.Time      `json:"completedAt,omitzero"`
	Quality     string         `json:"quality"`
	Ext         string         `json:"ext"`
}

func (it *DownloadItem) AsTrack() music.Track {
	tr := it.Track
	tr.ID = it.ID
	tr.Source = "download"
	return tr
}

type downloadManager struct {
	a        *app
	mu       sync.RWMutex
	items    []*DownloadItem
	active   int
	maxConns int
}

func newDownloadManager(a *app) *downloadManager {
	dm := &downloadManager{
		a:        a,
		maxConns: 2,
	}
	dm.load()
	return dm
}

func (dm *downloadManager) path() string {
	dir, err := mygo.App.Path(mygo.PathUserData)
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "downloads.json")
}

func (dm *downloadManager) load() {
	p := dm.path()
	if p == "" {
		return
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return
	}
	var loaded []*DownloadItem
	if err := json.Unmarshal(b, &loaded); err != nil {
		return
	}
	dm.mu.Lock()
	defer dm.mu.Unlock()
	dm.items = loaded
	for _, it := range dm.items {
		if it.Status == DownloadDone {
			if fi, err := os.Stat(it.Path); err != nil || fi.Size() == 0 {
				it.TotalBytes = 0
			} else {
				it.BytesDone = fi.Size()
				it.TotalBytes = fi.Size()
				it.Progress = 1.0
			}
		} else if it.Status == DownloadActive || it.Status == DownloadQueued {
			it.Status = DownloadFailed
			it.Error = "Download interrupted"
		}
	}
}

func (dm *downloadManager) save() {
	p := dm.path()
	if p == "" {
		return
	}
	dm.mu.RLock()
	data, err := json.MarshalIndent(dm.items, "", "  ")
	dm.mu.RUnlock()
	if err != nil {
		return
	}
	_ = os.MkdirAll(filepath.Dir(p), 0o755)
	_ = os.WriteFile(p, data, 0o644)
}

func (a *app) downloadDir() string {
	if a.settings.DownloadDir != "" {
		if err := os.MkdirAll(a.settings.DownloadDir, 0o755); err == nil {
			return a.settings.DownloadDir
		}
	}
	dir, err := mygo.App.Path(mygo.PathDownloads)
	if err != nil || dir == "" {
		dir, _ = os.UserHomeDir()
		dir = filepath.Join(dir, "Downloads")
	}
	target := filepath.Join(dir, "MyGo Music")
	_ = os.MkdirAll(target, 0o755)
	return target
}

func generateID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func (dm *downloadManager) Queue(tr music.Track, quality string) {
	dm.mu.Lock()
	for _, it := range dm.items {
		if sameTrack(&it.Track, &tr) {
			if it.Status == DownloadDone {
				if fi, err := os.Stat(it.Path); err == nil && fi.Size() > 0 {
					dm.mu.Unlock()
					dm.a.post(func() {
						dm.a.toast = Tf("Already downloaded %s", tr.Name)
					})
					return
				}
			} else if it.Status == DownloadActive || it.Status == DownloadQueued {
				dm.mu.Unlock()
				dm.a.post(func() {
					dm.a.toast = Tf("Already downloading %s", tr.Name)
				})
				return
			}
		}
	}

	id := generateID()
	item := &DownloadItem{
		ID:        id,
		Track:     tr,
		Quality:   quality,
		Status:    DownloadQueued,
		CreatedAt: time.Now(),
	}
	dm.items = append([]*DownloadItem{item}, dm.items...)
	dm.save()
	dm.mu.Unlock()

	dm.a.post(func() {
		dm.a.toast = Tf("Added %s to downloads", tr.Name)
		dm.a.redraw()
	})

	dm.processQueue()
}

func (dm *downloadManager) processQueue() {
	dm.mu.Lock()
	if dm.active >= dm.maxConns {
		dm.mu.Unlock()
		return
	}

	var next *DownloadItem
	for _, it := range dm.items {
		if it.Status == DownloadQueued {
			next = it
			break
		}
	}
	if next == nil {
		dm.mu.Unlock()
		return
	}

	next.Status = DownloadActive
	next.Error = ""
	dm.active++
	dm.save()
	dm.mu.Unlock()

	dm.a.post(func() {
		dm.a.redraw()
	})

	go dm.downloadWorker(next)
}

func (dm *downloadManager) downloadWorker(item *DownloadItem) {
	defer func() {
		dm.mu.Lock()
		dm.active--
		dm.save()
		dm.mu.Unlock()
		dm.a.post(func() {
			dm.a.redraw()
		})
		dm.processQueue()
	}()

	a := dm.a
	lib := a.trackLib(&item.Track)
	if lib == nil || lib.src == nil {
		item.Status = DownloadFailed
		item.Error = "No source available"
		return
	}

	quality := item.Quality
	if quality == "" {
		quality = a.settings.Quality
	}
	song, err := a.pl.FindSong(lib.src, item.Track, quality)
	if err != nil {
		item.Status = DownloadFailed
		item.Error = err.Error()
		a.post(func() {
			a.toast = Tf("Download failed: %s", item.Track.Name)
		})
		return
	}

	ext := song.Type
	item.Ext = ext
	dlDir := a.downloadDir()
	fileName := songFileName(&item.Track, ext)
	targetPath := filepath.Join(dlDir, fileName)

	item.Path = targetPath
	var lastUIUpdate time.Time

	err = song.SaveToWithProgress(targetPath, func(done, total int64) {
		dm.mu.Lock()
		if item.Status == DownloadCancelled {
			dm.mu.Unlock()
			return
		}
		item.BytesDone = done
		if total > 0 {
			item.TotalBytes = total
			item.Progress = float32(done) / float32(total)
		}
		dm.mu.Unlock()

		now := time.Now()
		if now.Sub(lastUIUpdate) >= 120*time.Millisecond {
			lastUIUpdate = now
			a.post(func() {
				if a.route.page == pageDownloads {
					a.redraw()
				}
			})
		}
	})

	dm.mu.Lock()
	if item.Status == DownloadCancelled {
		dm.mu.Unlock()
		_ = os.Remove(targetPath)
		return
	}

	if err != nil {
		item.Status = DownloadFailed
		item.Error = err.Error()
		dm.mu.Unlock()
		a.post(func() {
			a.toast = Tf("Download failed: %s", item.Track.Name)
			a.redraw()
		})
		return
	}

	// Download companion LRC lyrics if available
	if lib.can("lyrics") {
		if lyr, lerr := lib.src.Lyrics(item.Track.ID); lerr == nil && lyr != nil && lyr.LRC != "" {
			lrcPath := strings.TrimSuffix(targetPath, filepath.Ext(targetPath)) + ".lrc"
			_ = os.WriteFile(lrcPath, []byte(lyr.LRC), 0o644)
		}
	}

	if fi, serr := os.Stat(targetPath); serr == nil {
		item.BytesDone = fi.Size()
		item.TotalBytes = fi.Size()
	}
	item.Progress = 1.0
	item.Status = DownloadDone
	item.CompletedAt = time.Now()
	dm.mu.Unlock()

	a.post(func() {
		a.toast = Tf("Downloaded %s", item.Track.Name)
		a.redraw()
	})
}

func (dm *downloadManager) Cancel(id string) {
	dm.mu.Lock()
	for _, it := range dm.items {
		if it.ID == id {
			if it.Status == DownloadActive || it.Status == DownloadQueued {
				it.Status = DownloadCancelled
			}
			break
		}
	}
	dm.save()
	dm.mu.Unlock()
	dm.a.redraw()
}

func (dm *downloadManager) Retry(id string) {
	dm.mu.Lock()
	for _, it := range dm.items {
		if it.ID == id {
			it.Status = DownloadQueued
			it.Progress = 0
			it.BytesDone = 0
			it.Error = ""
			break
		}
	}
	dm.save()
	dm.mu.Unlock()
	dm.a.redraw()
	dm.processQueue()
}

func (dm *downloadManager) Delete(id string, deleteFile bool) {
	dm.mu.Lock()
	var path string
	for i, it := range dm.items {
		if it.ID == id {
			path = it.Path
			dm.items = append(dm.items[:i], dm.items[i+1:]...)
			break
		}
	}
	dm.save()
	dm.mu.Unlock()

	if deleteFile && path != "" {
		_ = os.Remove(path)
		_ = os.Remove(strings.TrimSuffix(path, filepath.Ext(path)) + ".lrc")
	}
	dm.a.redraw()
}

func (dm *downloadManager) ClearCompleted() {
	dm.mu.Lock()
	var remaining []*DownloadItem
	for _, it := range dm.items {
		if it.Status != DownloadDone {
			remaining = append(remaining, it)
		}
	}
	dm.items = remaining
	dm.save()
	dm.mu.Unlock()
	dm.a.redraw()
}

func (dm *downloadManager) ActiveCount() int {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	count := 0
	for _, it := range dm.items {
		if it.Status == DownloadActive || it.Status == DownloadQueued {
			count++
		}
	}
	return count
}

func (dm *downloadManager) CompletedItems() []*DownloadItem {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	var list []*DownloadItem
	for _, it := range dm.items {
		if it.Status == DownloadDone {
			list = append(list, it)
		}
	}
	return list
}

func (dm *downloadManager) ActiveAndQueuedItems() []*DownloadItem {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	var list []*DownloadItem
	for _, it := range dm.items {
		if it.Status == DownloadActive || it.Status == DownloadQueued || it.Status == DownloadFailed {
			list = append(list, it)
		}
	}
	return list
}

func (dm *downloadManager) Get(id string) *DownloadItem {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	for _, it := range dm.items {
		if it.ID == id {
			return it
		}
	}
	return nil
}

func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

// unsupportedSource provides dummy implementations of music.Source methods.
type unsupportedSource struct{}

func (unsupportedSource) Setting(string) string                                             { return "" }
func (unsupportedSource) SetSetting(string, string) error                                   { return music.ErrUnsupported }
func (unsupportedSource) SignIn() (*music.Account, error)                                   { return nil, music.ErrUnsupported }
func (unsupportedSource) SignOut() error                                                    { return music.ErrUnsupported }
func (unsupportedSource) Account() (*music.Account, error)                                   { return nil, music.ErrUnsupported }
func (unsupportedSource) Search(string, music.SearchKind, int) (*music.SearchResult, error) { return nil, music.ErrUnsupported }
func (unsupportedSource) Suggest(string) ([]string, error)                                  { return nil, music.ErrUnsupported }
func (unsupportedSource) Playlists() ([]music.Playlist, error)                              { return nil, music.ErrUnsupported }
func (unsupportedSource) Playlist(string) (*music.Playlist, error)                          { return nil, music.ErrUnsupported }
func (unsupportedSource) Album(string) (*music.AlbumPage, error)                            { return nil, music.ErrUnsupported }
func (unsupportedSource) Artist(string) (*music.ArtistPage, error)                          { return nil, music.ErrUnsupported }
func (unsupportedSource) Liked(int) (*music.Page, error)                                    { return nil, music.ErrUnsupported }
func (unsupportedSource) Recent() ([]music.Track, error)                                    { return nil, music.ErrUnsupported }
func (unsupportedSource) Home() (*music.Browse, error)                                      { return nil, music.ErrUnsupported }
func (unsupportedSource) Browse(string) (*music.Browse, error)                              { return nil, music.ErrUnsupported }
func (unsupportedSource) IsLiked([]string) (map[string]bool, error)                         { return nil, music.ErrUnsupported }
func (unsupportedSource) Like(string, bool) error                                           { return music.ErrUnsupported }
func (unsupportedSource) LikeEpisode(string, bool) error                                    { return music.ErrUnsupported }
func (unsupportedSource) CreatePlaylist(string) (*music.Playlist, error)                    { return nil, music.ErrUnsupported }
func (unsupportedSource) RenamePlaylist(string, string) error                               { return music.ErrUnsupported }
func (unsupportedSource) DeletePlaylist(string) error                                       { return music.ErrUnsupported }
func (unsupportedSource) AddToPlaylist(string, []string) error                              { return music.ErrUnsupported }
func (unsupportedSource) RemoveFromPlaylist(string, []string) error                         { return music.ErrUnsupported }
func (unsupportedSource) MovePlaylistTrack(string, int, int) error                          { return music.ErrUnsupported }
func (unsupportedSource) SavePlaylist(string, bool) error                                   { return music.ErrUnsupported }
func (unsupportedSource) FollowArtist(string, bool) error                                   { return music.ErrUnsupported }
func (unsupportedSource) SaveAlbum(string, bool) error                                      { return music.ErrUnsupported }
func (unsupportedSource) SavedAlbums(int) (*music.AlbumList, error)                         { return nil, music.ErrUnsupported }
func (unsupportedSource) Shows() ([]music.Show, error)                                      { return nil, music.ErrUnsupported }
func (unsupportedSource) Show(string, int) (*music.ShowPage, error)                         { return nil, music.ErrUnsupported }
func (unsupportedSource) SubscribeShow(string, bool) error                                  { return music.ErrUnsupported }
func (unsupportedSource) Stations() ([]music.Station, error)                                { return nil, music.ErrUnsupported }
func (unsupportedSource) StationNext(string) ([]music.Track, error)                         { return nil, music.ErrUnsupported }
func (unsupportedSource) Comments(string, int) (*music.CommentPage, error)                  { return nil, music.ErrUnsupported }
func (unsupportedSource) LikeComment(string, string, bool) error                            { return music.ErrUnsupported }
func (unsupportedSource) PostComment(string, string) (*music.Comment, error)                { return nil, music.ErrUnsupported }
func (unsupportedSource) RemotePlay([]string, int, int64) error                             { return music.ErrUnsupported }
func (unsupportedSource) RemotePause() error                                                { return music.ErrUnsupported }
func (unsupportedSource) RemoteResume() error                                               { return music.ErrUnsupported }
func (unsupportedSource) RemoteSeek(int64) error                                            { return music.ErrUnsupported }
func (unsupportedSource) RemoteVolume(int) error                                            { return music.ErrUnsupported }
func (unsupportedSource) RemoteState() (*music.RemoteState, error)                          { return nil, music.ErrUnsupported }
func (unsupportedSource) RemoteDevices() ([]music.Device, error)                            { return nil, music.ErrUnsupported }
func (unsupportedSource) RemoteUse(string) error                                            { return music.ErrUnsupported }

// downloadSource serves downloaded songs to the player.
type downloadSource struct {
	unsupportedSource
	mgr *downloadManager
}

func (s *downloadSource) Info() music.Info {
	return music.Info{
		ID:       "download",
		Name:     "Downloads",
		Playback: music.PlaybackStream,
	}
}

func (s *downloadSource) Can(what string) bool {
	switch what {
	case "stream", "lyrics":
		return true
	}
	return false
}

func (s *downloadSource) Stream(id, quality string) (*music.Stream, error) {
	it := s.mgr.Get(id)
	if it == nil || it.Path == "" {
		return nil, music.ErrUnavailable
	}
	f, err := os.Open(it.Path)
	if err != nil {
		return nil, music.ErrUnavailable
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return nil, music.ErrUnavailable
	}
	ext := it.Ext
	if ext == "" {
		ext = strings.TrimPrefix(filepath.Ext(it.Path), ".")
	}
	return &music.Stream{
		URL:     music.FileURL(it.Path),
		Type:    ext,
		Quality: "lossless",
		Size:    info.Size(),
	}, nil
}

func (s *downloadSource) Lyrics(id string) (*music.Lyrics, error) {
	it := s.mgr.Get(id)
	if it == nil || it.Path == "" {
		return &music.Lyrics{}, nil
	}
	lrcPath := strings.TrimSuffix(it.Path, filepath.Ext(it.Path)) + ".lrc"
	if b, err := os.ReadFile(lrcPath); err == nil && len(b) > 0 {
		return &music.Lyrics{LRC: string(b)}, nil
	}
	return &music.Lyrics{}, nil
}
