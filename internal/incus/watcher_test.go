package incus

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestWatcher_DebouncesAndTriggers(t *testing.T) {
	client := eventsTestServer(t, []string{
		`{"type":"lifecycle","project":"default","metadata":{"action":"instance-started","source":"/1.0/instances/a"}}`,
		`{"type":"lifecycle","project":"default","metadata":{"action":"instance-started","source":"/1.0/instances/b"}}`,
		`{"type":"lifecycle","project":"default","metadata":{"action":"instance-started","source":"/1.0/instances/c"}}`,
	})

	var calls int32
	fired := make(chan struct{}, 1)
	w := NewWatcher(client, func() {
		atomic.AddInt32(&calls, 1)
		select {
		case fired <- struct{}{}:
		default:
		}
	}, WithWatcherConfig(WatcherConfig{
		DebounceInterval:  100 * time.Millisecond,
		ReconnectInterval: 50 * time.Millisecond,
	}))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := w.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if !w.IsRunning() {
		t.Fatal("watcher should be running after Start")
	}

	select {
	case <-fired:
	case <-time.After(3 * time.Second):
		t.Fatal("reconcile was not triggered")
	}

	// The three events arrive in a burst well within the 100ms debounce, so
	// they should collapse into a single reconcile.
	time.Sleep(300 * time.Millisecond)
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Errorf("expected 1 debounced reconcile, got %d", got)
	}

	w.Stop()
	if w.IsRunning() {
		t.Error("watcher should not be running after Stop")
	}
}

func TestWatcher_StartIdempotent(t *testing.T) {
	client := eventsTestServer(t, nil)
	w := NewWatcher(client, func() {})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_ = w.Start(ctx)
	// Second Start is a no-op and must not panic or spawn a second loop.
	_ = w.Start(ctx)
	w.Stop()
}

func TestWatcher_DefaultsApplied(t *testing.T) {
	client := eventsTestServer(t, nil)
	w := NewWatcher(client, func() {}, WithWatcherConfig(WatcherConfig{}))
	if len(w.config.EventTypes) != 1 || w.config.EventTypes[0] != EventTypeLifecycle {
		t.Errorf("expected default event types [lifecycle], got %v", w.config.EventTypes)
	}
}

// TestWatcher_ConcurrentStopSafe ensures Stop is safe to call while events are
// in flight.
func TestWatcher_ConcurrentStopSafe(t *testing.T) {
	client := eventsTestServer(t, []string{
		`{"type":"lifecycle","project":"default","metadata":{"action":"instance-started","source":"/1.0/instances/a"}}`,
	})
	w := NewWatcher(client, func() {}, WithWatcherConfig(WatcherConfig{
		DebounceInterval:  10 * time.Millisecond,
		ReconnectInterval: 10 * time.Millisecond,
	}))
	ctx := context.Background()
	_ = w.Start(ctx)

	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			w.Stop()
		}()
	}
	wg.Wait()
}
