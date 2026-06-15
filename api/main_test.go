package main

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

// TestEveryTick_CallsFnOnTick verifies the helper invokes fn repeatedly on its
// interval. A non-blocking send keeps the spawned goroutine from ever blocking
// on a full channel, and the 1s timeout per tick is generous vs the 1ms
// interval so the test in not timing-flaky.
func TestEveryTick_CallsFnOnTick(t *testing.T) {
	ctx := t.Context()

	fired := make(chan struct{}, 1)
	everyTick(ctx, time.Millisecond, func(context.Context) {
		select {
		case fired <- struct{}{}:
		default:
		}
	})

	for i := range 3 {
		select {
		case <-fired:
		case <-time.After(time.Second):
			t.Fatalf("tick %d did not fire within 1s", i+1)
		}
	}
}

// TestEveryTick_StopsOnContextCancel verifies cancelling ctx stops the loop.
// Once cancel propagates the call count must stop growing. This is the property
// that lets the reconciler goroutines exit on graceful shutdown
func TestEveryTick_StopsOnContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	var calls int64
	started := make(chan struct{}, 1)
	everyTick(ctx, time.Millisecond, func(context.Context) {
		atomic.AddInt64(&calls, 1)
		select {
		case started <- struct{}{}:
		default:
		}
	})

	// Wait until the loop is actually running before cancelling.
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatalf("everyTick never called fn")
	}

	cancel()
	time.Sleep(20 * time.Millisecond) // let cancel land + any in-flight tick finish
	stable := atomic.LoadInt64(&calls)

	time.Sleep(30 * time.Millisecond) // 30 more ticks would land if still running
	if final := atomic.LoadInt64(&calls); final != stable {
		t.Fatalf("everyTick kept calling fn after cancel: %d -> %d", stable, final)
	}
}

func TestEveryTick_PassesContextToFn(t *testing.T) {
	type ctxKey string
	const marker ctxKey = "marker"
	ctx, cancel := context.WithCancel(context.WithValue(context.Background(), marker, "present"))
	defer cancel()

	got := make(chan string, 1)
	everyTick(ctx, time.Millisecond, func(c context.Context) {
		v, _ := c.Value(marker).(string)
		select {
		case got <- v:
		default:
		}
	})

	select {
	case v := <-got:
		if v != "present" {
			t.Fatalf("fn received wrong context: value = %q, want %q", v, "present")
		}
	case <-time.After(time.Second):
		t.Fatalf("fn never ran")
	}
}
