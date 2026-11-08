//go:build windows

package app

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"github.com/egoist/mygo"
)

func openInFileManager(path string) {
	if path == "" {
		dir, _ := mygo.App.Path(mygo.PathDownloads)
		if dir == "" {
			dir, _ = os.UserHomeDir()
			dir = filepath.Join(dir, "Downloads")
		}
		path = filepath.Join(dir, "MyGo Music")
	}

	cleanPath := filepath.Clean(path)

	// 1. If file exists, select and highlight it in Windows Explorer
	if fi, err := os.Stat(cleanPath); err == nil {
		if !fi.IsDir() {
			cmd := exec.Command("explorer")
			cmd.SysProcAttr = &syscall.SysProcAttr{
				CmdLine: fmt.Sprintf(`explorer.exe /select,"%s"`, cleanPath),
			}
			_ = cmd.Start()
			return
		}
		// 2. If directory exists, open directory
		cmd := exec.Command("explorer")
		cmd.SysProcAttr = &syscall.SysProcAttr{
			CmdLine: fmt.Sprintf(`explorer.exe "%s"`, cleanPath),
		}
		_ = cmd.Start()
		return
	}

	// 3. If file does not exist, check parent folder
	parent := filepath.Dir(cleanPath)
	if fi, err := os.Stat(parent); err == nil && fi.IsDir() {
		cmd := exec.Command("explorer")
		cmd.SysProcAttr = &syscall.SysProcAttr{
			CmdLine: fmt.Sprintf(`explorer.exe "%s"`, parent),
		}
		_ = cmd.Start()
		return
	}

	// 4. Default fallback: user Downloads\MyGo Music folder
	dir, _ := mygo.App.Path(mygo.PathDownloads)
	if dir == "" {
		dir, _ = os.UserHomeDir()
		dir = filepath.Join(dir, "Downloads")
	}
	mygoDir := filepath.Join(dir, "MyGo Music")
	_ = os.MkdirAll(mygoDir, 0o755)
	cmd := exec.Command("explorer")
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CmdLine: fmt.Sprintf(`explorer.exe "%s"`, mygoDir),
	}
	_ = cmd.Start()
}
