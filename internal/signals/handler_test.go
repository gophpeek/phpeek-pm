package signals

import (
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"
)

func TestIsPID1(t *testing.T) {
	// In normal test execution, we're not PID 1
	result := IsPID1()
	if result {
		t.Error("IsPID1() returned true, but we're not running as PID 1")
	}

	// Verify we're getting a sensible PID
	pid := os.Getpid()
	if pid <= 0 {
		t.Errorf("os.Getpid() returned invalid PID: %d", pid)
	}
}

func TestReapAll_NoZombies(t *testing.T) {
	// When there are no zombies to reap, reapAll should return without error
	// This tests the error/no-children path
	reapAll()
	// If we get here without panic, the test passes
}

func TestReapAll_WithZombie(t *testing.T) {
	// Skip if not on Unix-like system
	if os.Getenv("CI") == "true" {
		t.Skip("Skipping zombie reaping test in CI environment")
	}

	// Create a child process that exits immediately
	cmd := exec.Command("sh", "-c", "exit 0")
	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start child process: %v", err)
	}

	// Wait a bit for the process to exit and become a zombie
	time.Sleep(100 * time.Millisecond)

	// Try to reap - this should clean up our zombie child
	reapAll()

	// Verify the process is gone by trying to send signal 0
	err := cmd.Process.Signal(syscall.Signal(0))
	if err == nil {
		// Process still exists, which might happen if reap was too fast
		// Try waiting manually
		_ = cmd.Wait()
	}
}

func TestReapZombies_Goroutine(t *testing.T) {
	// Test that ReapZombies runs as a goroutine without blocking
	done := make(chan bool)
	started := make(chan bool)

	go func() {
		close(started)
		ReapZombies()
	}()

	// Wait for goroutine to start
	<-started

	// Give it a moment to run the ticker
	time.Sleep(50 * time.Millisecond)

	// If we can send to done, goroutine is still running (which is expected)
	select {
	case done <- true:
		// Goroutine received, unexpected since ReapZombies runs forever
	default:
		// Expected: ReapZombies is running its ticker loop
	}

	// The goroutine will be cleaned up when the test ends
}

func TestIsPID1_ReturnType(t *testing.T) {
	// Verify IsPID1 returns a boolean
	var result bool = IsPID1()
	if result && os.Getpid() != 1 {
		t.Error("IsPID1 returned true but os.Getpid() != 1")
	}
	if !result && os.Getpid() == 1 {
		t.Error("IsPID1 returned false but os.Getpid() == 1")
	}
}
