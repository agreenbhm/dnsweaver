// Package incus event watcher.
//
// WorkloadWatcher monitors the Incus event stream (/1.0/events) and triggers
// reconciliation when instances change, giving near-instant DNS updates instead
// of waiting for the next poll tick. It mirrors the Docker watcher
// (internal/watcher): debounced triggering, automatic reconnect with backoff,
// and graceful shutdown via context cancellation.
//
// The watcher only *triggers* reconciliation; the WorkloadListerAdapter remains
// the source of truth for instance state. The periodic reconcile timer is kept
// as a safety net for any missed events.
package incus

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/maxfield-allison/dnsweaver/internal/metrics"
)

// ReconcileFunc is called when an Incus event requires reconciliation.
type ReconcileFunc func()

// WatcherConfig holds event watcher configuration.
type WatcherConfig struct {
	// DebounceInterval is how long to wait for additional events before
	// triggering reconciliation, coalescing bursts (e.g. `incus-compose up`
	// starting several instances). Default: 2s.
	DebounceInterval time.Duration

	// ReconnectInterval is how long to wait before reconnecting after a stream
	// error. Default: 5s.
	ReconnectInterval time.Duration

	// EventTypes restricts the subscription to these Incus event types. Default:
	// ["lifecycle"].
	EventTypes []string
}

// DefaultWatcherConfig returns a WatcherConfig with sensible defaults.
func DefaultWatcherConfig() WatcherConfig {
	return WatcherConfig{
		DebounceInterval:  2 * time.Second,
		ReconnectInterval: 5 * time.Second,
		EventTypes:        []string{EventTypeLifecycle},
	}
}

// WorkloadWatcher watches Incus events and triggers reconciliation on instance
// changes.
type WorkloadWatcher struct {
	client      *Client
	onReconcile ReconcileFunc
	config      WatcherConfig
	logger      *slog.Logger

	mu       sync.Mutex
	cancel   context.CancelFunc
	running  bool
	debounce *time.Timer
}

// WatcherOption is a functional option for configuring the WorkloadWatcher.
type WatcherOption func(*WorkloadWatcher)

// WithWatcherConfig sets the watcher configuration.
func WithWatcherConfig(cfg WatcherConfig) WatcherOption {
	return func(w *WorkloadWatcher) {
		w.config = cfg
	}
}

// WithWatcherLogger sets a custom logger.
func WithWatcherLogger(logger *slog.Logger) WatcherOption {
	return func(w *WorkloadWatcher) {
		if logger != nil {
			w.logger = logger
		}
	}
}

// NewWatcher creates a new Incus event watcher.
func NewWatcher(client *Client, onReconcile ReconcileFunc, opts ...WatcherOption) *WorkloadWatcher {
	w := &WorkloadWatcher{
		client:      client,
		onReconcile: onReconcile,
		config:      DefaultWatcherConfig(),
		logger:      slog.Default(),
	}
	for _, opt := range opts {
		opt(w)
	}
	if len(w.config.EventTypes) == 0 {
		w.config.EventTypes = []string{EventTypeLifecycle}
	}
	return w
}

// Start begins watching Incus events. Non-blocking: it starts a goroutine and
// returns immediately. Call Stop() to halt watching.
func (w *WorkloadWatcher) Start(ctx context.Context) error {
	w.mu.Lock()
	if w.running {
		w.mu.Unlock()
		return nil
	}
	ctx, w.cancel = context.WithCancel(ctx)
	w.running = true
	w.mu.Unlock()

	go w.watchLoop(ctx)

	w.logger.Info("incus event watcher started",
		slog.Duration("debounce", w.config.DebounceInterval),
		slog.Any("event_types", w.config.EventTypes),
	)
	return nil
}

// Stop halts the event watcher.
func (w *WorkloadWatcher) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.cancel != nil {
		w.cancel()
		w.cancel = nil
	}
	if w.debounce != nil {
		w.debounce.Stop()
		w.debounce = nil
	}
	w.running = false
	w.logger.Info("incus event watcher stopped")
}

// IsRunning returns whether the watcher is currently running.
func (w *WorkloadWatcher) IsRunning() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.running
}

func (w *WorkloadWatcher) watchLoop(ctx context.Context) {
	defer func() {
		w.mu.Lock()
		w.running = false
		w.mu.Unlock()
	}()

	for {
		select {
		case <-ctx.Done():
			return
		default:
			err := w.client.StreamEvents(ctx, w.config.EventTypes, w.handleEvent)
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				w.logger.Warn("incus event stream error, reconnecting",
					slog.String("error", err.Error()),
					slog.Duration("retry_in", w.config.ReconnectInterval),
				)
				metrics.IncusWatcherReconnects.Inc()
				select {
				case <-ctx.Done():
					return
				case <-time.After(w.config.ReconnectInterval):
				}
			}
		}
	}
}

func (w *WorkloadWatcher) handleEvent(ev Event) {
	action := ev.Action()
	w.logger.Debug("received incus event",
		slog.String("type", ev.Type),
		slog.String("action", action),
		slog.String("project", ev.Project),
	)
	label := action
	if label == "" {
		label = ev.Type
	}
	metrics.IncusEventsProcessed.WithLabelValues(label).Inc()

	// Debounce: reset the timer on each event so bursts collapse into one
	// reconcile.
	w.mu.Lock()
	if w.debounce != nil {
		w.debounce.Stop()
	}
	w.debounce = time.AfterFunc(w.config.DebounceInterval, w.triggerReconcile)
	w.mu.Unlock()
}

func (w *WorkloadWatcher) triggerReconcile() {
	w.logger.Info("triggering reconciliation due to incus event")
	if w.onReconcile != nil {
		w.onReconcile()
	}
}
