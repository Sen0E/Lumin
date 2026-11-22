package translation

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"lumin/internal/domain"
)

type fake struct {
	id string
	fn func(context.Context) (string, error)
}

func (f fake) ID() string                                                    { return f.id }
func (f fake) Name() string                                                  { return f.id }
func (f fake) Translate(c context.Context, _ domain.Request) (string, error) { return f.fn(c) }

func TestParallelProvidersAndFailureIsolation(t *testing.T) {
	ready := make(chan struct{}, 2)
	release := make(chan struct{})
	providers := []domain.Translator{
		fake{"one", func(context.Context) (string, error) { ready <- struct{}{}; <-release; return "ok", nil }},
		fake{"two", func(context.Context) (string, error) { ready <- struct{}{}; <-release; return "", errors.New("failed") }},
	}
	go func() { <-ready; <-ready; close(release) }()
	var emitted atomic.Int32
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	results, err := Run(ctx, domain.Request{Text: "hello", Source: "auto", Target: "zh"}, providers, func(domain.Result) { emitted.Add(1) })
	if err != nil || len(results) != 2 || results[0].Text != "ok" || results[1].Error != "failed" || emitted.Load() != 2 {
		t.Fatalf("%+v %v", results, err)
	}
}

func TestCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	p := fake{"wait", func(ctx context.Context) (string, error) { cancel(); <-ctx.Done(); return "", ctx.Err() }}
	_, err := Run(ctx, domain.Request{Text: "x", Source: "en", Target: "zh"}, []domain.Translator{p}, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("%v", err)
	}
}
