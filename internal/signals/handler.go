package signals

import (
	"log/slog"
	"os"
	"syscall"
	"time"
)

// ReapZombies continuously reaps zombie processes.
// This is critical when running as PID 1 in a container.
// Without this, defunct processes accumulate and can exhaust PIDs.
func ReapZombies() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		reapAll()
	}
}

// reapAll reaps all zombie child processes
func reapAll() {
	for {
		var status syscall.WaitStatus
		pid, err := syscall.Wait4(-1, &status, syscall.WNOHANG, nil)

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

// IsPID1 returns true if the current process is PID 1
func IsPID1() bool {
	return os.Getpid() == 1
}
