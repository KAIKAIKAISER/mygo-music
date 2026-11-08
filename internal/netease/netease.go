package netease

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// MatchResult is the NetEase Cloud Music search match result.
type MatchResult struct {
	SongID      int64
	Title       string
	Artist      string
	CoverURL    string
	LyricsLRC   string
	Translation string
}

var (
	cacheMu sync.RWMutex
	cache   = make(map[string]*MatchResult)
	client  = &http.Client{Timeout: 6 * time.Second}
)

// Match searches NetEase Cloud Music by artist and song title,
// returning the best match with album cover URL and timed LRC lyrics.
func Match(artist, title string) *MatchResult {
	artist = strings.TrimSpace(artist)
	title = strings.TrimSpace(title)
	if title == "" {
		return nil
	}
	cacheKey := strings.ToLower(artist + " - " + title)
	cacheMu.RLock()
	if res, ok := cache[cacheKey]; ok {
		cacheMu.RUnlock()
		return res
	}
	cacheMu.RUnlock()

	res := fetchMatch(artist, title)
	if res != nil {
		cacheMu.Lock()
		cache[cacheKey] = res
		cacheMu.Unlock()
	}
	return res
}

func fetchMatch(artist, title string) *MatchResult {
	query := title
	if artist != "" && artist != "Unknown Artist" {
		query = artist + " " + title
	}

	searchURL := "http://music.163.com/api/cloudsearch/pc?s=" + url.QueryEscape(query) + "&type=1&offset=0&limit=3"
	req, err := http.NewRequest("GET", searchURL, nil)
	if err != nil {
		return nil
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Referer", "http://music.163.com/")

	resp, err := client.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	var searchData struct {
		Result struct {
			Songs []struct {
				ID   int64  `json:"id"`
				Name string `json:"name"`
				Ar   []struct {
					Name string `json:"name"`
				} `json:"ar"`
				Al struct {
					Name   string `json:"name"`
					PicURL string `json:"picUrl"`
				} `json:"al"`
			} `json:"songs"`
		} `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&searchData); err != nil {
		return nil
	}
	if len(searchData.Result.Songs) == 0 {
		return nil
	}

	song := searchData.Result.Songs[0]
	res := &MatchResult{
		SongID:   song.ID,
		Title:    song.Name,
		CoverURL: song.Al.PicURL,
	}
	if len(song.Ar) > 0 {
		res.Artist = song.Ar[0].Name
	}

	// Fetch lyrics & translations
	lyricURL := fmt.Sprintf("https://music.163.com/api/song/lyric?os=pc&id=%d&lv=-1&kv=-1&tv=-1", song.ID)
	reqL, err := http.NewRequest("GET", lyricURL, nil)
	if err == nil {
		reqL.Header.Set("User-Agent", "Mozilla/5.0")
		reqL.Header.Set("Referer", "https://music.163.com/")
		if respL, err := client.Do(reqL); err == nil {
			defer respL.Body.Close()
			var ldata struct {
				Lrc struct {
					Lyric string `json:"lyric"`
				} `json:"lrc"`
				TLyric struct {
					Lyric string `json:"lyric"`
				} `json:"tlyric"`
			}
			if json.NewDecoder(respL.Body).Decode(&ldata) == nil {
				res.LyricsLRC = ldata.Lrc.Lyric
				res.Translation = ldata.TLyric.Lyric
			}
		}
	}

	return res
}
