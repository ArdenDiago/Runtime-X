# Deferred Items - Phase 07

## Pre-existing Flaky Tests

**Tests:** `TestStart_CapturesOutput`, `TestStart_CapturesStderr`
**File:** `internal/scheduler/lifecycle_test.go`
**Discovered during:** Plan 02 execution (baseline test run)
**Issue:** These tests fail intermittently (~1 in 10 runs) when log capture goroutines haven't drained output before the test reads `mp.logs`. The test reads logs immediately after Start() returns without polling for content.
**Impact:** Not related to Phase 7 changes. Pre-existed before this phase.
**Recommended fix:** Add a short poll loop in `TestStart_CapturesOutput` and `TestStart_CapturesStderr` (similar to `waitForState`) that waits up to 100ms for at least one log entry to appear before asserting.
**Deferred to:** A suitable test-hardening pass or Phase 8 entry.
