// Package ocr defines the OCR adapter boundary and a fixed diagnostic image.
// The desktop application resolves engines from installed .luminext plugins.
package ocr

import (
	"context"
	_ "embed"
)

type Recognizer interface {
	Recognize(context.Context, string, string) (string, error)
}

// ProbeImage contains only the public text "Lumin translation test.".
//
//go:embed testdata/english.png
var ProbeImage []byte
