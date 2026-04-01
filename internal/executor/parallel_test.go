package executor

import (
	"bytes"
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkerPool_ConcurrencyLimit(t *testing.T) {
	const forks = 3
	const totalHosts = 10

	pool := NewWorkerPool(forks)
	var concurrent int64
	var maxConcurrent int64

	for i := 0; i < totalHosts; i++ {
		host := fmt.Sprintf("host-%d", i)
		pool.Submit(context.Background(), func(ctx context.Context) *HostResult {
			cur := atomic.AddInt64(&concurrent, 1)
			// Track maximum concurrency
			for {
				old := atomic.LoadInt64(&maxConcurrent)
				if cur <= old || atomic.CompareAndSwapInt64(&maxConcurrent, old, cur) {
					break
				}
			}

			time.Sleep(10 * time.Millisecond) // simulate work
			atomic.AddInt64(&concurrent, -1)

			return &HostResult{Host: host, Success: true}
		})
	}

	results := pool.Wait()

	assert.Equal(t, totalHosts, len(results))
	assert.LessOrEqual(t, atomic.LoadInt64(&maxConcurrent), int64(forks),
		"concurrency should not exceed forks limit")
	assert.Greater(t, atomic.LoadInt64(&maxConcurrent), int64(1),
		"should have some concurrent execution")
}

func TestWorkerPool_SingleFork(t *testing.T) {
	pool := NewWorkerPool(1)
	var concurrent int64
	var maxConcurrent int64

	for _, name := range []string{"a", "b", "c"} {
		name := name
		pool.Submit(context.Background(), func(ctx context.Context) *HostResult {
			cur := atomic.AddInt64(&concurrent, 1)
			for {
				old := atomic.LoadInt64(&maxConcurrent)
				if cur <= old || atomic.CompareAndSwapInt64(&maxConcurrent, old, cur) {
					break
				}
			}
			time.Sleep(10 * time.Millisecond)
			atomic.AddInt64(&concurrent, -1)
			return &HostResult{Host: name, Success: true}
		})
	}

	results := pool.Wait()
	assert.Equal(t, 3, len(results))
	// With forks=1, only one host should run at a time
	assert.Equal(t, int64(1), atomic.LoadInt64(&maxConcurrent))
}

func TestFlushBuffers_OrderedOutput(t *testing.T) {
	hosts := []string{"host-a", "host-b", "host-c"}

	// Simulate host-c finishing first, then host-a, then host-b
	results := []*HostResult{
		{Host: "host-c", Success: true, Output: bytes.NewBufferString("output-c\n")},
		{Host: "host-a", Success: true, Output: bytes.NewBufferString("output-a\n")},
		{Host: "host-b", Success: true, Output: bytes.NewBufferString("output-b\n")},
	}

	var buf bytes.Buffer
	FlushBuffers(&buf, hosts, results)

	expected := "output-a\noutput-b\noutput-c\n"
	assert.Equal(t, expected, buf.String(),
		"output should be flushed in host order, not completion order")
}

func TestFlushBuffers_EmptyBuffer(t *testing.T) {
	hosts := []string{"host-a"}
	results := []*HostResult{
		{Host: "host-a", Success: true, Output: &bytes.Buffer{}},
	}

	var buf bytes.Buffer
	FlushBuffers(&buf, hosts, results)

	assert.Empty(t, buf.String())
}

func TestWorkerPool_ErrorAggregation(t *testing.T) {
	pool := NewWorkerPool(5)

	pool.Submit(context.Background(), func(ctx context.Context) *HostResult {
		return &HostResult{Host: "host-a", Success: true}
	})
	pool.Submit(context.Background(), func(ctx context.Context) *HostResult {
		return &HostResult{Host: "host-b", Success: false, Error: fmt.Errorf("connection refused")}
	})
	pool.Submit(context.Background(), func(ctx context.Context) *HostResult {
		return &HostResult{Host: "host-c", Success: true}
	})

	results := pool.Wait()
	require.Equal(t, 3, len(results))

	// All hosts should have results regardless of individual failures
	var succeeded, failed int
	for _, r := range results {
		if r.Success {
			succeeded++
		} else {
			failed++
		}
	}
	assert.Equal(t, 2, succeeded)
	assert.Equal(t, 1, failed)
}

func TestWorkerPool_ContextCancellation(t *testing.T) {
	pool := NewWorkerPool(2)
	ctx, cancel := context.WithCancel(context.Background())

	var started int64

	// Submit tasks that block until cancelled
	for i := 0; i < 5; i++ {
		host := fmt.Sprintf("host-%d", i)
		pool.Submit(ctx, func(ctx context.Context) *HostResult {
			atomic.AddInt64(&started, 1)
			<-ctx.Done()
			return &HostResult{Host: host, Success: false, Error: ctx.Err()}
		})
	}

	// Give workers time to start
	time.Sleep(50 * time.Millisecond)

	// Cancel context
	cancel()

	results := pool.Wait()

	// All 5 tasks should have results (either ran or got cancelled waiting for semaphore)
	assert.Equal(t, 5, len(results))

	// All should report failure
	for _, r := range results {
		assert.False(t, r.Success)
		assert.Error(t, r.Error)
	}
}

func TestBufferedWriter_ThreadSafe(t *testing.T) {
	bw := &BufferedWriter{}
	done := make(chan bool, 10)

	// Concurrent writes
	for i := 0; i < 10; i++ {
		go func(n int) {
			_, err := fmt.Fprintf(bw, "line %d\n", n)
			assert.NoError(t, err)
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	content := string(bw.Bytes())
	// Should have 10 lines
	lines := 0
	for _, c := range content {
		if c == '\n' {
			lines++
		}
	}
	assert.Equal(t, 10, lines)
}

func TestNewWorkerPool_MinForks(t *testing.T) {
	pool := NewWorkerPool(0)
	assert.Equal(t, 1, pool.forks, "forks should be at least 1")

	pool = NewWorkerPool(-5)
	assert.Equal(t, 1, pool.forks, "negative forks should be clamped to 1")
}
