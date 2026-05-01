package executor

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tackhq/tack/internal/connector"
	"github.com/tackhq/tack/internal/output"
	"github.com/tackhq/tack/internal/playbook"
)

// fakeConnector returns canned fact-gather output for tests.
type fakeConnector struct {
	host         string
	connectErr   error
	executeErr   error
	executeDelay time.Duration

	connectCount int64
	closeCount   int64
}

func (f *fakeConnector) Connect(ctx context.Context) error {
	atomic.AddInt64(&f.connectCount, 1)
	return f.connectErr
}

func (f *fakeConnector) Execute(ctx context.Context, cmd string) (*connector.Result, error) {
	if f.executeDelay > 0 {
		select {
		case <-time.After(f.executeDelay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if f.executeErr != nil {
		return nil, f.executeErr
	}
	// Synthetic facts output mirroring the factsScript line format.
	stdout := strings.Join([]string{
		"TACK_FACT os_type=Linux",
		"TACK_FACT architecture=x86_64",
		"TACK_FACT kernel=6.1.0-test",
		"TACK_FACT hostname=" + f.host,
		"TACK_FACT user=test",
		"TACK_FACT home=/root",
	}, "\n") + "\n"
	return &connector.Result{Stdout: stdout, ExitCode: 0}, nil
}

func (f *fakeConnector) Upload(ctx context.Context, src io.Reader, dst string, mode uint32) error {
	return nil
}
func (f *fakeConnector) Download(ctx context.Context, src string, dst io.Writer) error { return nil }
func (f *fakeConnector) SetSudo(enabled bool, password string)                          {}
func (f *fakeConnector) Close() error {
	atomic.AddInt64(&f.closeCount, 1)
	return nil
}
func (f *fakeConnector) String() string { return "fake://" + f.host }

func newFakeExecutor(factory func(play *playbook.Play, host string) (connector.Connector, error)) *Executor {
	e := New()
	e.Output = output.New(&bytes.Buffer{})
	e.connectorFactory = factory
	return e
}

func TestGatherFactsParallel_ConcurrentExecution(t *testing.T) {
	const numHosts = 10

	hosts := make([]string, numHosts)
	for i := range hosts {
		hosts[i] = fmt.Sprintf("host-%d", i)
	}

	var concurrent int64
	var maxConcurrent int64
	e := newFakeExecutor(func(play *playbook.Play, host string) (connector.Connector, error) {
		return &fakeConnector{host: host, executeDelay: 30 * time.Millisecond}, nil
	})

	// Wrap factory to record concurrency at Execute time. We use a sync hook
	// inside Execute via a custom fakeConnector subtype.
	e.connectorFactory = func(play *playbook.Play, host string) (connector.Connector, error) {
		return &concurrentTrackingFake{
			fakeConnector: fakeConnector{host: host, executeDelay: 30 * time.Millisecond},
			concurrent:    &concurrent,
			maxConcurrent: &maxConcurrent,
		}, nil
	}

	play := &playbook.Play{Hosts: hosts, Connection: "ssh"}
	preps := e.gatherFactsParallel(context.Background(), play)

	require.NotNil(t, preps)
	assert.Equal(t, numHosts, len(preps))

	// Each host should have a distinct facts map keyed by its hostname.
	for _, host := range hosts {
		prep := preps[host]
		require.NotNil(t, prep, "missing prep for %s", host)
		require.NoError(t, prep.err, "host %s err: %v", host, prep.err)
		require.NotNil(t, prep.facts)
		assert.Equal(t, host, prep.facts["hostname"])
	}

	// Concurrency must exceed 1 (we set executeDelay so jobs overlap).
	// We don't assert == numHosts because factsConcurrency caps at 20 and the
	// scheduler may not start all jobs simultaneously.
	assert.Greater(t, atomic.LoadInt64(&maxConcurrent), int64(1),
		"expected concurrent fact gather; observed max=%d", maxConcurrent)
}

type concurrentTrackingFake struct {
	fakeConnector
	concurrent    *int64
	maxConcurrent *int64
}

func (f *concurrentTrackingFake) Execute(ctx context.Context, cmd string) (*connector.Result, error) {
	cur := atomic.AddInt64(f.concurrent, 1)
	defer atomic.AddInt64(f.concurrent, -1)
	for {
		old := atomic.LoadInt64(f.maxConcurrent)
		if cur <= old || atomic.CompareAndSwapInt64(f.maxConcurrent, old, cur) {
			break
		}
	}
	return f.fakeConnector.Execute(ctx, cmd)
}

func TestGatherFactsParallel_SkipConditions(t *testing.T) {
	factory := func(play *playbook.Play, host string) (connector.Connector, error) {
		return &fakeConnector{host: host}, nil
	}

	t.Run("local connection", func(t *testing.T) {
		e := newFakeExecutor(factory)
		play := &playbook.Play{Hosts: []string{"a", "b"}, Connection: "local"}
		assert.Nil(t, e.gatherFactsParallel(context.Background(), play))
	})

	t.Run("single host", func(t *testing.T) {
		e := newFakeExecutor(factory)
		play := &playbook.Play{Hosts: []string{"only"}, Connection: "ssh"}
		assert.Nil(t, e.gatherFactsParallel(context.Background(), play))
	})

	t.Run("zero hosts", func(t *testing.T) {
		e := newFakeExecutor(factory)
		play := &playbook.Play{Hosts: nil, Connection: "ssh"}
		assert.Nil(t, e.gatherFactsParallel(context.Background(), play))
	})

	t.Run("gather facts disabled", func(t *testing.T) {
		e := newFakeExecutor(factory)
		gather := false
		play := &playbook.Play{
			Hosts:       []string{"a", "b"},
			Connection:  "ssh",
			GatherFacts: &gather,
		}
		assert.Nil(t, e.gatherFactsParallel(context.Background(), play))
	})
}

func TestGatherFactsParallel_PerHostFailureIsolation(t *testing.T) {
	failHost := "host-2"
	factory := func(play *playbook.Play, host string) (connector.Connector, error) {
		if host == failHost {
			return &fakeConnector{host: host, executeErr: fmt.Errorf("simulated gather failure")}, nil
		}
		return &fakeConnector{host: host}, nil
	}

	e := newFakeExecutor(factory)
	play := &playbook.Play{
		Hosts:      []string{"host-1", "host-2", "host-3"},
		Connection: "ssh",
	}
	preps := e.gatherFactsParallel(context.Background(), play)
	require.NotNil(t, preps)

	require.NoError(t, preps["host-1"].err)
	require.NotNil(t, preps["host-1"].conn)

	require.Error(t, preps["host-2"].err, "host-2 should have a pre-pass error")
	assert.Nil(t, preps["host-2"].conn, "failed host should release connector")

	require.NoError(t, preps["host-3"].err)
	require.NotNil(t, preps["host-3"].conn)

	// Cleanup successful preps so the test doesn't leak fake connectors
	// (verifies Close is callable without panic).
	closePrepConnectors(preps)
}

func TestFlushPrepBuffers_OrderedOutput(t *testing.T) {
	hosts := []string{"host-a", "host-b", "host-c"}

	preps := map[string]*hostPrep{
		"host-c": {host: "host-c", output: bytes.NewBufferString("output-c\n")},
		"host-a": {host: "host-a", output: bytes.NewBufferString("output-a\n")},
		"host-b": {host: "host-b", output: bytes.NewBufferString("output-b\n")},
	}

	var buf bytes.Buffer
	flushPrepBuffers(&buf, hosts, preps)

	assert.Equal(t, "output-a\noutput-b\noutput-c\n", buf.String(),
		"flush should follow play.Hosts order, not completion order")
}

func TestGatherFactsParallel_ContextCancellation(t *testing.T) {
	// Each fake gather sleeps long enough that cancellation should bite.
	factory := func(play *playbook.Play, host string) (connector.Connector, error) {
		return &fakeConnector{host: host, executeDelay: 500 * time.Millisecond}, nil
	}
	e := newFakeExecutor(factory)
	play := &playbook.Play{
		Hosts:      []string{"a", "b", "c", "d", "e"},
		Connection: "ssh",
	}

	ctx, cancel := context.WithCancel(context.Background())

	var preps map[string]*hostPrep
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		preps = e.gatherFactsParallel(ctx, play)
	}()

	// Give the workers a moment to start, then cancel.
	time.Sleep(50 * time.Millisecond)
	cancel()

	wg.Wait()
	require.NotNil(t, preps)

	// At least some hosts should have observed the cancellation. We don't
	// require all of them — the scheduler may have completed a few before
	// cancel landed — but the cumulative wait must be much shorter than
	// the un-cancelled total of 5 * 500ms = 2.5s.
	cancelledCount := 0
	for _, prep := range preps {
		if prep.err != nil && strings.Contains(prep.err.Error(), context.Canceled.Error()) {
			cancelledCount++
		}
	}
	assert.GreaterOrEqual(t, cancelledCount, 1,
		"expected at least one host to observe context cancellation; preps=%v", preps)
}

func TestGatherFactsParallel_OutputBuffersInHostOrder(t *testing.T) {
	// Stagger Execute durations so completion order != host order, then
	// confirm the flushed output still matches the host-order layout.
	factory := func(play *playbook.Play, host string) (connector.Connector, error) {
		var d time.Duration
		switch host {
		case "host-a":
			d = 150 * time.Millisecond
		case "host-b":
			d = 50 * time.Millisecond
		case "host-c":
			d = 100 * time.Millisecond
		}
		return &fakeConnector{host: host, executeDelay: d}, nil
	}
	e := newFakeExecutor(factory)
	hosts := []string{"host-a", "host-b", "host-c"}
	play := &playbook.Play{Hosts: hosts, Connection: "ssh"}

	preps := e.gatherFactsParallel(context.Background(), play)
	require.NotNil(t, preps)

	var buf bytes.Buffer
	flushPrepBuffers(&buf, hosts, preps)

	out := buf.String()
	idxA := strings.Index(out, "host-a")
	idxB := strings.Index(out, "host-b")
	idxC := strings.Index(out, "host-c")

	require.Greater(t, idxA, -1, "expected host-a in flushed output: %q", out)
	require.Greater(t, idxB, -1, "expected host-b in flushed output: %q", out)
	require.Greater(t, idxC, -1, "expected host-c in flushed output: %q", out)
	assert.True(t, idxA < idxB && idxB < idxC,
		"flushed output should be in host order; got idxA=%d idxB=%d idxC=%d",
		idxA, idxB, idxC)

	// Sanity: cleanup connectors.
	closePrepConnectors(preps)
}

// ansiPattern matches CSI escape sequences (e.g. "\x1b[1m" or "\x1b[0m").
// Used by tests that assert on visible output regardless of color setting.
var ansiPattern = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func stripANSI(s string) string {
	return ansiPattern.ReplaceAllString(s, "")
}

func TestGatherFactsParallel_BannerInlinedPerHost(t *testing.T) {
	// Each host's buffered emitter should produce a single-line HOST banner
	// with the fact-gathering result inlined, terminated by a newline.
	factory := func(play *playbook.Play, host string) (connector.Connector, error) {
		return &fakeConnector{host: host, executeDelay: 5 * time.Millisecond}, nil
	}
	e := newFakeExecutor(factory)
	hosts := []string{"web1", "web2", "web3"}
	play := &playbook.Play{Hosts: hosts, Connection: "ssh"}

	preps := e.gatherFactsParallel(context.Background(), play)
	require.NotNil(t, preps)

	var buf bytes.Buffer
	flushPrepBuffers(&buf, hosts, preps)

	// Strip ANSI escape codes for stable substring matching across
	// color-enabled and color-disabled executor configurations.
	out := stripANSI(buf.String())
	for _, h := range hosts {
		// Each host's banner must appear as a single line ending in
		// "gathering facts ✓\n".
		want := "HOST " + h + " [ssh] - gathering facts ✓\n"
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in flushed output; got %q", want, out)
		}
	}
	// No legacy "Gathering Facts" task lines should be present.
	if strings.Contains(out, "Gathering Facts") {
		t.Errorf("legacy 'Gathering Facts' task line should be retired; got %q", out)
	}

	closePrepConnectors(preps)
}
