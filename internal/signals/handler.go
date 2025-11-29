package signals

import (
	"log/slog"
	"os"
	"sync"
	"syscall"
	"time"
)

// WaitFunc is the function signature for syscall.Wait4
// Allows mocking in tests
type WaitFunc func(pid int, wstatus *syscall.WaitStatus, options int, rusage *syscall.Rusage) (wpid int, err error)

// waitFunc is the function used for waiting on child processes
// Can be replaced in tests for mocking
var waitFunc WaitFunc = syscall.Wait4
var waitFuncMu sync.RWMutex

// getWaitFunc returns the current wait function with proper synchronization
func getWaitFunc() WaitFunc {
	waitFuncMu.RLock()
	defer waitFuncMu.RUnlock()
	return waitFunc
}

// setWaitFunc sets the wait function with proper synchronization (for testing)
func setWaitFunc(f WaitFunc) {
	waitFuncMu.Lock()
	defer waitFuncMu.Unlock()
	waitFunc = f
}

// ReapZombies continuously reaps zombie processes.
// This is critical when running as PID 1 in a container.
// Without this, defunct processes accumulate and can exhaust PIDs.
// The interval parameter controls how often zombie reaping occurs.
// If interval is 0 or negative, it defaults to 1 second.
func ReapZombies(interval time.Duration) {
	if interval <= 0 {
		interval = 1 * time.Second // Default to 1 second if not configured
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		reapAll()
	}
}

// reapAll reaps all zombie child processes
func reapAll() {
	waitFn := getWaitFunc()
	for {
		var status syscall.WaitStatus
		pid, err := waitFn(-1, &status, syscall.WNOHANG, nil)

		if err != nil || pid <= 0 {
			// No more zombies to reap
			break
		}

		slog.Debug("Reaped zombie process",
			"pid", pid,
			"status", status,
		)
	}
}

// ReapCount returns the number of zombies reaped in a single pass
// Useful for testing and monitoring
func ReapCount() int {
	waitFn := getWaitFunc()
	count := 0
	for {
		var status syscall.WaitStatus
		pid, err := waitFn(-1, &status, syscall.WNOHANG, nil)

		if err != nil || pid <= 0 {
			break
		}

		count++
		slog.Debug("Reaped zombie process",
			"pid", pid,
			"status", status,
		)
	}
	return count
}

// IsPID1 returns true if the current process is PID 1
func IsPID1() bool {
	return os.Getpid() == 1
}
