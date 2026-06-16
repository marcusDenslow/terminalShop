package main

import (
	"context"
	"testing"
	"time"
)

// TestEveryTick_CallsFnOnTick verifies the helper invokes fn repeatedly on its
// interval. A non-blocking send keeps the spawned goroutine from ever blocking
// on a full channel, and the 1s timeout per tick is generous vs the 1ms
// interval so the test is not timing-flaky.
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
// After the first tick we cancel inside the inter-tick gap, so everyTick's
// select sees only ctx.Done ready (ticker.C is cold) and returns
// deterministically — there is no random pick between two ready cases. We then
// confirm no further tick arrives within a full interval, which a live loop
// could not satisfy. This is the shutdown guarantee the reconcilers depend on.
func TestEveryTick_StopsOnContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	const interval = 50 * time.Millisecond
	ticks := make(chan struct{}) // unbuffered: fn blocks until the test receives
	everyTick(ctx, interval, func(context.Context) {
		ticks <- struct{}{}
	})

	// First tick proves the loop is running and leaves the goroutine back in
	// select with ticker.C cold for the next ~interval.
	select {
	case <-ticks:
	case <-time.After(time.Second):
		t.Fatal("everyTick never called fn")
	}

	cancel()

	// Cancel landed in the cold gap, so the loop must have taken ctx.Done and
	// returned. A live loop would tick again at ~interval; a stopped one never
	// will.
	select {
	case <-ticks:
		t.Fatal("everyTick ticked again after cancel")
	case <-time.After(interval + 50*time.Millisecond):
		// good: silent past a full interval — the loop returned
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
