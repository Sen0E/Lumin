// Package translation coordinates independent providers without depending on UI.
package translation

import (
	"context"
	"errors"
	"sync"
	"time"

	"lumin/internal/domain"
)

// Run calls providers concurrently, returning results in configured order. emit is
// serialized, but runs on worker goroutines: callers must dispatch UI mutations.
func Run(ctx context.Context, req domain.Request, providers []domain.Translator, emit func(domain.Result)) ([]domain.Result, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	if len(providers) == 0 {
		return nil, errors.New("请先在「翻译服务」中配置并启用至少一个服务")
	}
	results := make([]domain.Result, len(providers))
	var wg sync.WaitGroup
	var notify sync.Mutex
	sem := make(chan struct{}, 4)
	for i, p := range providers {
		wg.Go(func() {
			r := domain.Result{ProviderID: p.ID(), ProviderName: p.Name()}
			start := time.Now()
			select {
			case sem <- struct{}{}:
				text, err := p.Translate(ctx, req)
				<-sem
				if err != nil {
					r.Error = err.Error()
				} else {
					r.Text = text
				}
			case <-ctx.Done():
				r.Error = ctx.Err().Error()
			}
			r.DurationMS = time.Since(start).Milliseconds()
			results[i] = r
			if emit != nil {
				notify.Lock()
				emit(r)
				notify.Unlock()
			}
		})
	}
	wg.Wait()
	return results, ctx.Err()
}
