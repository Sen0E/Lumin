package selection

import (
	"log"
	"os"
	"time"
)

// Trace records stages and errors only. Never log the selected text itself.
// The existing capture switch also enables this for current diagnostic runs.
func Trace(stage string, elapsed time.Duration, err error) {
	if os.Getenv("LUMIN_SELECTION_TRACE") != "1" && os.Getenv("LUMIN_CAPTURE_TRACE") != "1" {
		return
	}
	if err != nil {
		log.Printf("[selection] %s: %s; %v", stage, elapsed.Round(time.Millisecond), err)
		return
	}
	log.Printf("[selection] %s: %s; ok", stage, elapsed.Round(time.Millisecond))
}
