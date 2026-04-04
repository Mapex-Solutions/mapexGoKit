package health

import (
	"context"
	"sync"
	"time"
)

// Service aggregates health checkers and caches results.
type Service struct {
	cfg       Config
	checkers  []CheckerConfig
	startedAt time.Time

	mu          sync.RWMutex
	cachedResp  *Response
	lastCheckAt time.Time
}

// NewService creates a health service with the given config and checkers.
func NewService(cfg Config, checkers ...CheckerConfig) *Service {
	if cfg.CacheTTL == 0 {
		cfg.CacheTTL = 10 * time.Second
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 5 * time.Second
	}

	return &Service{
		cfg:       cfg,
		checkers:  checkers,
		startedAt: time.Now(),
	}
}

// Check runs all health checkers (with cache) and returns the aggregated result.
func (s *Service) Check(ctx context.Context) *Response {
	s.mu.RLock()
	if s.cachedResp != nil && time.Since(s.lastCheckAt) < s.cfg.CacheTTL {
		resp := s.cachedResp
		s.mu.RUnlock()
		return resp
	}
	s.mu.RUnlock()

	checkCtx, cancel := context.WithTimeout(ctx, s.cfg.Timeout)
	defer cancel()

	checks := make(map[string]CheckDetail, len(s.checkers))
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, cc := range s.checkers {
		wg.Add(1)
		go func(cc CheckerConfig) {
			defer wg.Done()
			hs := cc.Checker.Check(checkCtx)

			detail := CheckDetail{
				Connected:    hs.Connected,
				Critical:     cc.Critical,
				LatencyMs:    hs.LatencyMs,
				ErrorMessage: hs.ErrorMessage,
			}

			mu.Lock()
			checks[cc.Checker.Name()] = detail
			mu.Unlock()
		}(cc)
	}

	wg.Wait()

	status := "healthy"
	for _, d := range checks {
		if !d.Connected && d.Critical {
			status = "unhealthy"
			break
		}
		if !d.Connected {
			status = "degraded"
		}
	}

	now := time.Now()
	resp := &Response{
		Status:      status,
		Service:     s.cfg.ServiceName,
		Version:     s.cfg.Version,
		Uptime:      time.Since(s.startedAt).Round(time.Second).String(),
		Timestamp:   now,
		LastCheckAt: now,
		Checks:      checks,
	}

	s.mu.Lock()
	s.cachedResp = resp
	s.lastCheckAt = now
	s.mu.Unlock()

	return resp
}
