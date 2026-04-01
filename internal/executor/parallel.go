package executor

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sync"
)

// HostResult captures the outcome of executing a play against a single host.
type HostResult struct {
	Host    string
	Success bool
	Error   error
	Stats   Stats
	Output  *bytes.Buffer
}

// BufferedWriter wraps a bytes.Buffer to implement io.Writer for per-host output capture.
type BufferedWriter struct {
	buf bytes.Buffer
	mu  sync.Mutex
}

// Write appends data to the buffer in a thread-safe manner.
func (bw *BufferedWriter) Write(p []byte) (int, error) {
	bw.mu.Lock()
	defer bw.mu.Unlock()
	return bw.buf.Write(p)
}

// Bytes returns the buffered content.
func (bw *BufferedWriter) Bytes() []byte {
	bw.mu.Lock()
	defer bw.mu.Unlock()
	return bw.buf.Bytes()
}

// WorkerPool manages concurrent host execution with a semaphore-based limit.
type WorkerPool struct {
	forks   int
	sem     chan struct{}
	wg      sync.WaitGroup
	mu      sync.Mutex
	results []*HostResult
}

// NewWorkerPool creates a worker pool with the given concurrency limit.
func NewWorkerPool(forks int) *WorkerPool {
	if forks < 1 {
		forks = 1
	}
	return &WorkerPool{
		forks: forks,
		sem:   make(chan struct{}, forks),
	}
}

// Submit launches a host execution function in a goroutine, respecting the concurrency limit.
// The provided ctx is passed to the function and should be checked for cancellation.
func (wp *WorkerPool) Submit(ctx context.Context, fn func(ctx context.Context) *HostResult) {
	wp.wg.Add(1)
	go func() {
		defer wp.wg.Done()

		// Acquire semaphore slot
		select {
		case wp.sem <- struct{}{}:
			defer func() { <-wp.sem }()
		case <-ctx.Done():
			wp.addResult(&HostResult{
				Success: false,
				Error:   ctx.Err(),
			})
			return
		}

		result := fn(ctx)
		wp.addResult(result)
	}()
}

// Wait blocks until all submitted tasks complete and returns results in submission order.
func (wp *WorkerPool) Wait() []*HostResult {
	wp.wg.Wait()
	wp.mu.Lock()
	defer wp.mu.Unlock()
	return wp.results
}

func (wp *WorkerPool) addResult(r *HostResult) {
	wp.mu.Lock()
	defer wp.mu.Unlock()
	wp.results = append(wp.results, r)
}

// FlushBuffers writes buffered output from host results to the writer in the
// order of the hosts slice (not completion order).
func FlushBuffers(w io.Writer, hosts []string, results []*HostResult) {
	// Build lookup by host name
	byHost := make(map[string]*HostResult, len(results))
	for _, r := range results {
		byHost[r.Host] = r
	}

	for _, host := range hosts {
		r, ok := byHost[host]
		if !ok || r.Output == nil {
			continue
		}
		data := r.Output.Bytes()
		if len(data) > 0 {
			fmt.Fprint(w, string(data))
		}
	}
}
