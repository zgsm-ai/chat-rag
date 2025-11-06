package timeout

import (
	"context"
	"sync"
	"time"

	"github.com/zgsm-ai/chat-rag/internal/logger"
	"go.uber.org/zap"
)

// IdleTimeoutReason represents the reason for idle timeout
type IdleTimeoutReason string

const (
	IdleTimeoutReasonPerIdle IdleTimeoutReason = "per_idle"
	IdleTimeoutReasonTotal   IdleTimeoutReason = "total"
)

// IdleTracker maintains the total idle budget across retries/degradations
type IdleTracker struct {
	mu              sync.Mutex
	initialBudget   time.Duration
	remainingBudget time.Duration
	lastResetTime   time.Time
}

// NewIdleTracker creates a new IdleTracker with the specified total budget
func NewIdleTracker(totalBudget time.Duration) *IdleTracker {
	return &IdleTracker{
		initialBudget:   totalBudget,
		remainingBudget: totalBudget,
		lastResetTime:   time.Now(),
	}
}

// Remaining returns the remaining total idle budget
func (t *IdleTracker) Remaining() time.Duration {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.remainingBudget
}

// Consume reduces the remaining budget by the specified duration
func (t *IdleTracker) Consume(duration time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.remainingBudget -= duration
	if t.remainingBudget < 0 {
		t.remainingBudget = 0
	}
}

// Reset resets both the single idle timer and the total idle tracker
func (t *IdleTracker) Reset() {
	t.mu.Lock()
	defer t.mu.Unlock()
	// Reset total budget to initial value
	t.remainingBudget = t.initialBudget
	t.lastResetTime = time.Now()
}

// IdleTimer manages idle timeout for a single request attempt
type IdleTimer struct {
	ctx           context.Context
	cancel        context.CancelFunc
	perIdle       time.Duration
	tracker       *IdleTracker
	timer         *time.Timer
	mu            sync.Mutex
	reason        IdleTimeoutReason
	stopped       bool
	lastResetTime time.Time
	idleStartTime time.Time
	resetCount    int64
}

// NewIdleTimer creates a new IdleTimer with the specified per-idle timeout and tracker
// Returns the context, cancel function, and the timer instance
func NewIdleTimer(parentCtx context.Context, perIdle time.Duration, tracker *IdleTracker) (context.Context, context.CancelFunc, *IdleTimer) {
	ctx, cancel := context.WithCancel(parentCtx)

	it := &IdleTimer{
		ctx:           ctx,
		cancel:        cancel,
		perIdle:       perIdle,
		tracker:       tracker,
		reason:        IdleTimeoutReasonPerIdle,
		lastResetTime: time.Now(),
		idleStartTime: time.Now(),
		resetCount:    0,
	}

	// Check if total budget is already exhausted
	if tracker.Remaining() <= 0 {
		it.reason = IdleTimeoutReasonTotal
		cancel()
		logger.Info("IdleTimer: total budget exhausted at creation",
			zap.Duration("remaining", tracker.Remaining()))
		return ctx, cancel, it
	}

	// Always use perIdle as the initial window, regardless of remaining budget
	it.timer = time.NewTimer(perIdle)

	// Start watching in a goroutine
	go it.watch()

	logger.Info("IdleTimer created",
		zap.Duration("perIdle", perIdle),
		zap.Duration("totalRemaining", tracker.Remaining()))

	return ctx, cancel, it
}

// watch monitors the timer and contexts
func (it *IdleTimer) watch() {
	select {
	case <-it.ctx.Done():
		// Parent context cancelled
		it.mu.Lock()
		if it.timer != nil {
			it.timer.Stop()
		}
		it.mu.Unlock()
		return

	case <-it.timer.C:
		// Timer expired - handle timeout
		it.handleTimeout()
	}
}

// handleTimeout is called when the timer expires
func (it *IdleTimer) handleTimeout() {
	it.mu.Lock()
	defer it.mu.Unlock()

	if it.stopped {
		return
	}

	// Calculate actual idle duration since last reset
	actualIdleDuration := time.Since(it.idleStartTime)

	// Consume the perIdle duration from total budget
	it.tracker.Consume(it.perIdle)

	remaining := it.tracker.Remaining()

	logger.Warn("IdleTimer: timeout triggered",
		zap.Duration("perIdle", it.perIdle),
		zap.Duration("actualIdleDuration", actualIdleDuration),
		zap.Duration("remainingBudget", remaining),
		zap.Int64("resetCount", it.resetCount))

	// Check if total budget is exhausted
	if remaining <= 0 {
		it.reason = IdleTimeoutReasonTotal
		logger.Warn("IdleTimer: total budget exhausted",
			zap.Duration("consumed", it.tracker.initialBudget-remaining))
	} else {
		it.reason = IdleTimeoutReasonPerIdle
	}

	// Cancel the context
	it.cancel()
}

// Reset resets the idle timer when data is received
func (it *IdleTimer) Reset() {
	it.mu.Lock()
	defer it.mu.Unlock()

	if it.stopped {
		return
	}

	// Reset the tracker's total budget
	it.tracker.Reset()

	// Reset the timer to perIdle duration
	if it.timer != nil {
		it.timer.Stop()
		it.timer.Reset(it.perIdle)
	}

	it.lastResetTime = time.Now()
	it.idleStartTime = time.Now()
	it.resetCount++

	logger.Debug("IdleTimer reset",
		zap.Duration("perIdle", it.perIdle),
		zap.Duration("remainingBudget", it.tracker.Remaining()),
		zap.Int64("resetCount", it.resetCount))
}

// Reason returns the reason for the timeout
func (it *IdleTimer) Reason() IdleTimeoutReason {
	it.mu.Lock()
	defer it.mu.Unlock()
	return it.reason
}

// Stop stops the timer and prevents further timeouts
func (it *IdleTimer) Stop() {
	it.mu.Lock()
	defer it.mu.Unlock()

	if it.stopped {
		return
	}

	it.stopped = true
	if it.timer != nil {
		it.timer.Stop()
	}

	logger.Debug("IdleTimer stopped",
		zap.Int64("resetCount", it.resetCount),
		zap.Duration("remainingBudget", it.tracker.Remaining()))
}

// GetResetCount returns the number of times Reset was called
func (it *IdleTimer) GetResetCount() int64 {
	it.mu.Lock()
	defer it.mu.Unlock()
	return it.resetCount
}
