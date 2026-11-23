// A standalone OCR adapter illustrating the recognize operation.
package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"time"

	"lumin/pkg/pluginapi"
)

var languages = regexp.MustCompile(`^[a-zA-Z0-9_]+(?:\+[a-zA-Z0-9_]+)*$`)

type limited struct{ bytes.Buffer }

func (b *limited) Write(p []byte) (int, error) {
	if b.Len()+len(p) > 1<<20 {
		return 0, errors.New("OCR output too large")
	}
	return b.Buffer.Write(p)
}
func recognize(ctx context.Context, r pluginapi.Request) (string, error) {
	if r.Operation != "recognize" {
		return "", errors.New("unsupported operation")
	}
	if !languages.MatchString(r.Languages) {
		return "", errors.New("invalid OCR languages")
	}
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "tesseract", r.ImagePath, "stdout", "-l", r.Languages)
	var out, diagnostics limited
	cmd.Stdout = &out
	cmd.Stderr = &diagnostics
	if err := cmd.Run(); err != nil {
		return "", errors.New("Tesseract failed; check installation and language packs")
	}
	return out.String(), nil
}
func main() {
	if err := pluginapi.Serve(recognize); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
