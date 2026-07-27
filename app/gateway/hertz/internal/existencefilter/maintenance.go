package existencefilter

import (
	"context"
	"time"
)

func (f *RedisBitmap) Start(ctx context.Context, interval time.Duration, onError func(error)) {
	if interval <= 0 {
		interval = 6 * time.Hour
	}
	go func() {
		status := f.Status(ctx)
		if status.Ready {
			filterReady.WithLabelValues("product").Set(1)
			filterItems.WithLabelValues("product").Set(float64(status.Items))
		} else if rebuildErr := f.Rebuild(ctx); rebuildErr != nil && onError != nil {
			onError(rebuildErr)
		}
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := f.Rebuild(ctx); err != nil && onError != nil {
					onError(err)
				}
			}
		}
	}()
}
