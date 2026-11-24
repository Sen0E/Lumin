package bootstrap

import "time"

// All methods and dispatched callbacks run on main. The timer worker only
// enqueues work; revision rejects a callback already queued before refocus or
// a new selection request, even when stopping its timer is too late.
type translationBlur struct {
	revision uint64
	timer    *time.Timer
	dispatch func(func())
}

func newTranslationBlur(dispatch func(func())) *translationBlur {
	return &translationBlur{dispatch: dispatch}
}
func (b *translationBlur) cancel() {
	b.revision++
	if b.timer != nil {
		b.timer.Stop()
		b.timer = nil
	}
}
func (b *translationBlur) schedule(delay time.Duration, hide func()) {
	b.cancel()
	ticket := b.revision
	b.timer = time.AfterFunc(delay, func() {
		b.dispatch(func() {
			if ticket != b.revision {
				return
			}
			b.timer = nil
			hide()
		})
	})
}
