package ocr

import (
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestTesseractEnglishImage(t *testing.T) {
	if _, err := exec.LookPath("tesseract"); err != nil {
		t.Skip("Tesseract is not installed")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	languages, err := exec.CommandContext(ctx, "tesseract", "--list-langs").Output()
	if err != nil || !strings.Contains("\n"+string(languages), "\neng\n") {
		t.Skip("English OCR data is not installed")
	}
	text, err := (Tesseract{}).Recognize(ctx, "testdata/english.png", "eng")
	if err != nil || !strings.Contains(text, "Lumin translation test") {
		t.Fatalf("text %q: %v", text, err)
	}
}

func TestOutputLimit(t *testing.T) {
	b := &limitedBuffer{}
	if _, err := b.Write(make([]byte, 1<<20)); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Write([]byte("x")); err == nil {
		t.Fatal("output limit was not enforced")
	}
}
