## 1. Core Infrastructure

- [x] 1.1 Create `internal/executor/parallel.go` with worker pool type: semaphore channel, WaitGroup, HostResult collector
- [x] 1.2 Create `HostResult` struct to capture per-host success/failure, stats, and buffered output
- [x] 1.3 Create buffered output writer that implements `io.Writer` and captures output per-host

## 2. Executor Integration

- [x] 2.1 Add `Forks int` field to executor options/config
- [x] 2.2 Refactor host loop in `runPlay()` to use worker pool when forks > 1
- [x] 2.3 Pass per-host buffered writer to `runPlayOnHost()` instead of direct stdout
- [x] 2.4 Collect `HostResult` from each goroutine and aggregate into `RunResult`

## 3. Output Handling

- [x] 3.1 Implement sequential buffer flush — after all hosts complete, flush buffers in host order
- [x] 3.2 Bypass buffering when forks=1 (preserve existing real-time output behavior)
- [x] 3.3 Update recap display to show per-host status summary when forks > 1

## 4. CLI Wiring

- [x] 4.1 Wire existing `--forks` flag value to executor config in `cmd/bolt/main.go`
- [x] 4.2 Add `BOLT_FORKS` environment variable binding
- [x] 4.3 Validate forks value (must be >= 1)

## 5. Error Handling & Cancellation

- [x] 5.1 Implement error aggregation — collect per-host errors without stopping other hosts
- [x] 5.2 Wire context cancellation to all worker goroutines
- [x] 5.3 Ensure connector Close() is called in defer within each goroutine

## 6. Testing

- [x] 6.1 Unit test worker pool with mock connectors — verify concurrency limit respected
- [x] 6.2 Unit test output buffering — verify ordered flush
- [x] 6.3 Unit test error aggregation — verify one failure doesn't stop others
- [x] 6.4 Unit test context cancellation — verify all goroutines exit
