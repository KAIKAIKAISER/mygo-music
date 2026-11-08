package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo"
)

func TestDownloadDir(t *testing.T) {
	a := &app{
		postFn: func(fn func()) { fn() },
	}
	p, err := mygo.App.Path(mygo.PathDownloads)
	t.Logf("mygo.App.Path(mygo.PathDownloads): %q, err: %v", p, err)
	dl := a.downloadDir()
	t.Logf("a.downloadDir(): %q", dl)
	fi, err := os.Stat(dl)
	t.Logf("os.Stat(dl): %v, isDir: %v", err, fi != nil && fi.IsDir())
}

func TestOpenInFileManager(t *testing.T) {
	// Should not panic on empty string
	openInFileManager("")

	// Should not panic on valid directory
	dir, _ := mygo.App.Path(mygo.PathDownloads)
	openInFileManager(dir)

	// Should not panic on non-existent file with existing parent directory
	openInFileManager(filepath.Join(dir, "MyGo Music", "non_existent.flac"))
}
