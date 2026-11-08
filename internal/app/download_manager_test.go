package app

import (
	"testing"
	"time"

	"mygo-music/internal/music"
)

func TestDownloadManagerQueueDoesNotDeadlock(t *testing.T) {
	a := &app{
		postFn: func(fn func()) { fn() },
	}
	dm := newDownloadManager(a)

	tr := music.Track{
		ID:     "test-tr-1",
		Name:   "Test Song",
		Source: "test",
	}

	done := make(chan bool, 1)
	go func() {
		dm.Queue(tr, "lossless")
		done <- true
	}()

	select {
	case <-done:
		// Queue completed without deadlocking!
	case <-time.After(2 * time.Second):
		t.Fatal("dm.Queue deadlocked or timed out!")
	}

	if count := dm.ActiveCount(); count != 1 {
		t.Logf("ActiveCount is %d", count)
	}
}
