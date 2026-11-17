package netease

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
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

type nSong struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Ar   []struct {
		Name string `json:"name"`
	} `json:"ar"`
	Al struct {
		Name   string `json:"name"`
		PicURL string `json:"picUrl"`
	} `json:"al"`
}

var (
	cacheMu sync.RWMutex
	cache   = make(map[string]*MatchResult)
	client  = &http.Client{Timeout: 6 * time.Second}

	noiseRegex = regexp.MustCompile(`(?i)\s*[\(\[\{【](?:official\s*(?:music\s*)?(?:video|audio|mv|lyric\s*video|track|hd|4k|visualizer)?|mv|video|audio|lyrics?|full\s*(?:ver\.|version|audio)?|special\s*video|dance\s*practice|performance\s*video|m/v)[\)\]\}】]`)
	topicRegex = regexp.MustCompile(`(?i)\s*-\s*topic\b`)
)

func cleanTitle(title string) string {
	t := noiseRegex.ReplaceAllString(title, "")
	return strings.TrimSpace(t)
}

func cleanArtist(artist string) string {
	a := topicRegex.ReplaceAllString(artist, "")
	a = strings.TrimSuffix(a, " - Topic")
	return strings.TrimSpace(a)
}

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

func searchCandidates(query string) []nSong {
	searchURL := "http://music.163.com/api/cloudsearch/pc?s=" + url.QueryEscape(query) + "&type=1&offset=0&limit=5"
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
			Songs []nSong `json:"songs"`
		} `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&searchData); err != nil {
		return nil
	}
	return searchData.Result.Songs
}

func fetchLyrics(songID int64) (lrc string, trans string) {
	lyricURL := fmt.Sprintf("https://music.163.com/api/song/lyric?os=pc&id=%d&lv=-1&kv=-1&tv=-1", songID)
	reqL, err := http.NewRequest("GET", lyricURL, nil)
	if err != nil {
		return "", ""
	}
	reqL.Header.Set("User-Agent", "Mozilla/5.0")
	reqL.Header.Set("Referer", "https://music.163.com/")
	respL, err := client.Do(reqL)
	if err != nil {
		return "", ""
	}
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
		return ldata.Lrc.Lyric, ldata.TLyric.Lyric
	}
	return "", ""
}

func fetchMatch(artist, title string) *MatchResult {
	a := cleanArtist(artist)
	t := cleanTitle(title)

	// If title is "Artist - Song", and artist is generic or empty, extract real artist and title
	if strings.Contains(t, " - ") && (a == "" || strings.EqualFold(a, "unknown artist") || strings.EqualFold(a, "various artists")) {
		parts := strings.SplitN(t, " - ", 2)
		a = cleanArtist(parts[0])
		t = cleanTitle(parts[1])
	}

	var queries []string
	if a != "" && !strings.EqualFold(a, "unknown artist") {
		queries = append(queries, a+" "+t)
	}
	if t != "" {
		queries = append(queries, t)
	}
	if strings.Contains(title, " - ") {
		parts := strings.SplitN(title, " - ", 2)
		queries = append(queries, cleanTitle(parts[0])+" "+cleanTitle(parts[1]))
	}
	if artist != "" && title != "" {
		queries = append(queries, artist+" "+title)
	}

	seen := make(map[string]bool)
	var songs []nSong
	for _, q := range queries {
		q = strings.TrimSpace(q)
		if q == "" || seen[q] {
			continue
		}
		seen[q] = true
		songs = searchCandidates(q)
		if len(songs) > 0 {
			break
		}
	}

	if len(songs) == 0 {
		return nil
	}

	// Iterate candidates to find the first with timed lyrics
	var bestSong *nSong
	var bestLRC, bestTrans string
	for i := range songs {
		s := &songs[i]
		lrc, trans := fetchLyrics(s.ID)
		if lrc != "" {
			bestSong = s
			bestLRC = lrc
			bestTrans = trans
			break
		}
	}

	if bestSong == nil {
		bestSong = &songs[0]
	}

	res := &MatchResult{
		SongID:      bestSong.ID,
		Title:       bestSong.Name,
		CoverURL:    bestSong.Al.PicURL,
		LyricsLRC:   bestLRC,
		Translation: bestTrans,
	}
	if len(bestSong.Ar) > 0 {
		res.Artist = bestSong.Ar[0].Name
	}
	return res
}
