package bootstrap

import (
	"testing"
	"time"
)

func TestQueuedBlurCannotHideRefocusedOrNewSelectionWindow(t *testing.T) {
	q := make(chan func(), 4)
	b := newTranslationBlur(func(fn func()) { q <- fn })
	defer b.cancel()
	hidden := 0
	b.schedule(time.Millisecond, func() { hidden++ })
	var old func()
	select {
	case old = <-q:
	case <-time.After(time.Second):
		t.Fatal("blur did not enqueue")
	}
	b.cancel() // Refocus/new selection after the timer fired but before Update.
	old()
	if hidden != 0 {
		t.Fatal("queued old blur hid new interaction")
	}
	b.schedule(time.Millisecond, func() { hidden++ })
	select {
	case current := <-q:
		current()
	case <-time.After(time.Second):
		t.Fatal("new blur not dispatched")
	}
	if hidden != 1 {
		t.Fatal("current blur ignored")
	}
}
