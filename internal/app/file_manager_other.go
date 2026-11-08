//go:build !windows

package app

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

func openInFileManager(path string) {
	if path == "" {
		return
	}
	switch runtime.GOOS {
	case "darwin":
		if fi, err := os.Stat(path); err == nil && !fi.IsDir() {
			_ = exec.Command("open", "-R", path).Start()
		} else {
			_ = exec.Command("open", filepath.Dir(path)).Start()
		}
	default:
		if fi, err := os.Stat(path); err == nil && fi.IsDir() {
			_ = exec.Command("xdg-open", path).Start()
		} else {
			_ = exec.Command("xdg-open", filepath.Dir(path)).Start()
		}
	}
}
