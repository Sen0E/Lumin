package capture

import (
	"log"
	"os"
	"time"
)

// TracePhase records only a stage and duration, never pixels, paths or text.
// Enable explicitly to diagnose desktop portal vs application latency.
func TracePhase(stage string) func() {
	if os.Getenv("LUMIN_CAPTURE_TRACE") != "1" {
		return func() {}
	}
	start := time.Now()
	return func() { log.Printf("[capture] %s: %s", stage, time.Since(start).Round(time.Millisecond)) }
}
