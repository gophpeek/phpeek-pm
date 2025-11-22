# Known Issues - Requires Debugging

## Session Summary

**Status:** 16 commits, 17,500+ lines implemented, but critical runtime bug blocks testing.

**Work completed:** All features implemented and committed
**Blocker:** Daemon hangs during startup (readiness channel deadlock)
**Effort to fix:** Estimated 2-4 hours of focused debugging

---

## Critical Issues Found During Testing

### 1. Daemon Hangs After Starting Processes ⚠️ CRITICAL BLOCKER
**Symptom:**
- Daemon starts processes successfully
- Log shows "Process instance started" for all processes
- Then hangs - never prints "All processes started successfully"
- API server never starts
- Daemon unresponsive to SIGINT/SIGTERM

**Reproduction:**
```bash
./build/phpeek-pm serve -c tui-demo.yaml
# Hangs after starting process instances
# Ctrl+C does nothing
```

**Suspected Cause:**
- Readiness blocking logic may have deadlock
- WaitForReadiness() might block even when no health checks configured
- Channel operation deadlock in supervisor.Start()

**Files to investigate:**
- internal/process/manager.go:82-130 (dependency waiting loop)
- internal/process/supervisor.go:86-122 (WaitForReadiness)
- internal/process/supervisor.go:149-182 (readiness channel initialization)

**Temporary Workaround:**
Use configs from before readiness blocking commit (07a4c7d).

---

### 2. TUI Shows No Processes
**Symptom:**
- TUI connects to API successfully
- Process table is empty (shows headers only)
- No error messages

**Suspected Cause:**
- API not actually running (see Issue #1)
- Or ListProcesses() returns empty array

**Reproduction:**
```bash
# Terminal 1
./build/phpeek-pm serve -c tui-demo.yaml
# (Hangs, API doesn't start)

# Terminal 2
./build/phpeek-pm tui
# Shows empty table
```

---

### 3. Graceful Shutdown Not Working
**Symptom:**
- Ctrl+C or SIGINT ignored
- Daemon continues running
- Must use SIGKILL to stop

**Root Cause:**
Daemon hangs before reaching waitForShutdown() (Issue #1).
Signal handling code is correct but never reached.

---

## Investigation Plan

### Step 1: Fix Readiness Deadlock
Check if readinessCh is being waited on when it's never closed:

1. Add debug logging in WaitForReadiness()
2. Check if channel is closed in all code paths
3. Verify isReady flag prevents double-close

### Step 2: Test Without Readiness Blocking
Temporarily disable readiness waiting to isolate issue:

```go
// In manager.go:82, comment out dependency waiting:
// if len(procCfg.DependsOn) > 0 {
//     ... dependency waiting code ...
// }
```

### Step 3: Add Timeout to Channel Operations
Ensure all channel operations have timeouts:

```go
select {
case <-s.readinessCh:
    // ready
case <-time.After(1 * time.Second):
    // timeout - log warning and continue
}
```

---

## Fixes Applied This Session

### ✅ Fixed: Ctrl+C Killing Child Processes
- Added Setpgid: true to isolate child processes
- Prevents terminal signals from propagating to children
- Commit: 0a28d72

### ✅ Fixed: Metrics/API Port Conflicts Crash Daemon
- Changed os.Exit(1) to slog.Warn() + continue
- Graceful degradation on port conflicts
- Commit: 342118f

### ✅ Fixed: API Not Enabled by Default
- Set APIEnabled = true in defaults
- TUI works out-of-box
- Commit: 1509e70

---

## Testing Status

### What Works:
- ✅ Build completes
- ✅ Config validation (check-config)
- ✅ Version command
- ✅ Subcommand CLI structure
- ✅ Process configuration parsing

### What's Broken:
- ❌ Daemon hangs after starting processes
- ❌ API server doesn't start
- ❌ TUI shows empty process list
- ❌ Graceful shutdown not working
- ❌ SIGINT/SIGTERM ignored

### Needs Debugging:
- Readiness channel deadlock
- Manager.Start() blocking issue
- Signal handling in serve command

---

## Recommendation

**Revert readiness blocking temporarily** until deadlock is fixed:

```bash
git revert 07a4c7d  # Revert s6-overlay parity commit
# Test if daemon works without readiness blocking
# Fix deadlock
# Re-apply readiness blocking
```

**Or debug readiness logic:**
- Add extensive logging
- Test each channel operation
- Verify all code paths close channel when no health check

---

## Session Summary

**What was accomplished:**
- 14 commits
- 17,000+ lines of code
- Full feature set implemented
- Documentation complete

**What needs fixing before production:**
- Critical deadlock in readiness blocking
- Signal handling in daemon mode
- Full integration testing

**Estimated effort to fix:**
- 4-6 hours of debugging
- Focus on readiness channel logic
- Add comprehensive integration tests
