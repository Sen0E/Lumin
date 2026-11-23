package pluginapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestServeProtocol(t *testing.T) {
	for _, fail := range []bool{false, true} {
		var out bytes.Buffer
		err := ServeIO(strings.NewReader(`{"protocol":1,"id":"abc","operation":"translate","text":"hello","settings":{}}`), &out, func(ctx context.Context, r Request) (string, error) {
			if r.Text != "hello" {
				t.Fatal(r)
			}
			if fail {
				return "", errors.New("test failure")
			}
			return "你好", nil
		})
		if err != nil {
			t.Fatal(err)
		}
		var reply Response
		if err := json.Unmarshal(out.Bytes(), &reply); err != nil {
			t.Fatal(err)
		}
		if reply.ID != "abc" || reply.Protocol != 1 || (!fail && reply.Text != "你好") || (fail && reply.Error != "test failure") {
			t.Fatal(reply)
		}
	}
	for _, body := range []string{`{"protocol":2,"id":"a"}`, `{"protocol":1}`, `{} {}`, strings.Repeat("x", (4<<20)+1)} {
		if err := ServeIO(strings.NewReader(body), &bytes.Buffer{}, func(context.Context, Request) (string, error) {
			t.Fatal("invalid request reached handler")
			return "", nil
		}); err == nil {
			t.Fatal("accepted invalid protocol")
		}
	}
}
