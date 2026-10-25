package manager

import (
	"fmt"
	"golang.org/x/sys/windows"
	"os"
	"path/filepath"
	"time"
)

// Windows supports config/lock tests; kernel operations remain Linux-only.
func (m Manager) Lock() (func(), error) {
	p := m.Path("/run/greflow.lock")
	if e := os.MkdirAll(filepath.Dir(p), 0700); e != nil {
		return nil, e
	}
	f, e := os.OpenFile(p, os.O_CREATE|os.O_RDWR, 0600)
	if e != nil {
		return nil, e
	}
	wait := m.LockWait
	if wait == 0 {
		wait = 60 * time.Second
	}
	deadline := time.Now().Add(wait)
	var overlapped windows.Overlapped
	for {
		e = windows.LockFileEx(windows.Handle(f.Fd()), windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, &overlapped)
		if e == nil {
			break
		}
		if e != windows.ERROR_LOCK_VIOLATION || time.Now().After(deadline) {
			f.Close()
			return nil, fmt.Errorf("host lock: %w", e)
		}
		time.Sleep(20 * time.Millisecond)
	}
	return func() { windows.UnlockFileEx(windows.Handle(f.Fd()), 0, 1, 0, &overlapped); f.Close() }, nil
}
