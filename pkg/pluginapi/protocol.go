// Package pluginapi is the small, framework-independent Lumin plugin SDK.
package pluginapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"regexp"
	"strings"
)

const Version = 1

type Request struct {
	Protocol  int               `json:"protocol"`
	ID        string            `json:"id"`
	Operation string            `json:"operation"`
	Text      string            `json:"text,omitempty"`
	Source    string            `json:"source,omitempty"`
	Target    string            `json:"target,omitempty"`
	ImagePath string            `json:"image_path,omitempty"`
	Languages string            `json:"languages,omitempty"`
	Settings  map[string]string `json:"settings"`
}
type Response struct {
	Protocol int    `json:"protocol"`
	ID       string `json:"id"`
	Text     string `json:"text"`
	Error    string `json:"error,omitempty"`
}

var identifier = regexp.MustCompile(`^[a-z][a-z0-9.-]{0,79}$`)
var version = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+(?:-[a-zA-Z0-9.-]+)?$`)

func ValidRef(ref string) bool {
	id, v, ok := strings.Cut(ref, "@")
	return ok && identifier.MatchString(id) && !strings.Contains(id, "..") && len(v) <= 64 && version.MatchString(v)
}

// Serve handles exactly one request. Diagnostics belong on stderr, never stdout.
func Serve(handler func(context.Context, Request) (string, error)) error {
	return ServeIO(os.Stdin, os.Stdout, handler)
}
func ServeIO(in io.Reader, out io.Writer, handler func(context.Context, Request) (string, error)) error {
	b, err := io.ReadAll(io.LimitReader(in, (4<<20)+1))
	if err != nil {
		return err
	}
	if len(b) > 4<<20 {
		return errors.New("plugin request exceeds 4 MiB")
	}
	var request Request
	if err := json.Unmarshal(b, &request); err != nil {
		return err
	}
	if request.Protocol != Version || request.ID == "" {
		return errors.New("unsupported protocol or missing request ID")
	}
	text, err := handler(context.Background(), request)
	response := Response{Protocol: Version, ID: request.ID, Text: text}
	if err != nil {
		response.Text = ""
		response.Error = err.Error()
	}
	return json.NewEncoder(out).Encode(response)
}
