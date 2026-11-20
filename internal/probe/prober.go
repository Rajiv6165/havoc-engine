package probe

import (
	"context"
	"math"
	"net/http"
	"sync"
	"time"
)

// HTTPClient represents any client capable of executing HTTP requests.
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// Config specifies the runtime parameters for a Prober.
type Config struct {
	URL         string
	Interval    time.Duration // default: 250ms
	Timeout     time.Duration // default: 1s
	GracePeriod time.Duration // default: 5s
	Client      HTTPClient    // default: &http.Client{Timeout: cfg.Timeout}
}

// Result holds the final metrics recorded by a probing session.
type Result struct {
	TotalRequests       int        `json:"totalRequests"`
	FailedRequests      int        `json:"failedRequests"`
	DowntimeMs          int        `json:"downtimeMs"` // longest consecutive failure streak in milliseconds
	FirstFailureTime    *time.Time `json:"firstFailureTime,omitempty"`
	LastFailureTime     *time.Time `json:"lastFailureTime,omitempty"`
	AvailabilityPercent float64    `json:"availabilityPercent"`
}

// Prober actively probes a target HTTP endpoint at regular intervals.
type Prober struct {
	cfg    Config
	client HTTPClient
}

// NewProber initializes a Prober with the provided configuration and defaults.
func NewProber(cfg Config) *Prober {
	if cfg.Interval <= 0 {
		cfg.Interval = 250 * time.Millisecond
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 1 * time.Second
	}
	if cfg.GracePeriod < 0 {
		cfg.GracePeriod = 0
	} else if cfg.GracePeriod == 0 {
		cfg.GracePeriod = 5 * time.Second
	}

	client := cfg.Client
	if client == nil {
		client = &http.Client{
			Timeout: cfg.Timeout,
		}
	}

	return &Prober{
		cfg:    cfg,
		client: client,
	}
}

// Start launches active probing in a background goroutine.
// It returns a stop function that triggers the grace period, waits for probing to conclude,
// and returns the final Result. If URL is empty, probing is skipped and stop returns nil.
func (p *Prober) Start(ctx context.Context) func() *Result {
	if p == nil || p.cfg.URL == "" {
		return func() *Result { return nil }
	}

	stopCh := make(chan struct{})
	doneCh := make(chan *Result, 1)

	go func() {
		doneCh <- p.run(ctx, stopCh)
	}()

	var (
		once sync.Once
		res  *Result
	)
	return func() *Result {
		once.Do(func() {
			close(stopCh)
			select {
			case res = <-doneCh:
			case <-ctx.Done():
			}
		})
		return res
	}
}

func (p *Prober) run(ctx context.Context, stopCh <-chan struct{}) *Result {
	ticker := time.NewTicker(p.cfg.Interval)
	defer ticker.Stop()

	var (
		totalRequests  int
		failedRequests int
		currentStreak  int
		maxStreak      int
		firstFailure   *time.Time
		lastFailure    *time.Time
		graceTimer     *time.Timer
		graceCh        <-chan time.Time
	)

	probeOnce := func() {
		totalRequests++
		reqCtx, reqCancel := context.WithTimeout(ctx, p.cfg.Timeout)
		defer reqCancel()

		req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, p.cfg.URL, nil)
		failed := false
		if err != nil {
			failed = true
		} else {
			resp, err := p.client.Do(req)
			if err != nil {
				failed = true
			} else {
				_ = resp.Body.Close()
				if resp.StatusCode < 200 || resp.StatusCode >= 300 {
					failed = true
				}
			}
		}

		now := time.Now()
		if failed {
			failedRequests++
			currentStreak++
			if currentStreak > maxStreak {
				maxStreak = currentStreak
			}
			if firstFailure == nil {
				t := now
				firstFailure = &t
			}
			t := now
			lastFailure = &t
		} else {
			currentStreak = 0
		}
	}

	// First probe immediately upon starting
	probeOnce()

	for {
		select {
		case <-ctx.Done():
			return p.buildResult(totalRequests, failedRequests, maxStreak, firstFailure, lastFailure)

		case <-stopCh:
			stopCh = nil // disable branch so it only fires once
			if p.cfg.GracePeriod > 0 {
				graceTimer = time.NewTimer(p.cfg.GracePeriod)
				defer graceTimer.Stop()
				graceCh = graceTimer.C
			} else {
				return p.buildResult(totalRequests, failedRequests, maxStreak, firstFailure, lastFailure)
			}

		case <-graceCh:
			return p.buildResult(totalRequests, failedRequests, maxStreak, firstFailure, lastFailure)

		case <-ticker.C:
			probeOnce()
		}
	}
}

func (p *Prober) buildResult(total, failed, maxStreak int, first, last *time.Time) *Result {
	downtimeMs := maxStreak * int(p.cfg.Interval/time.Millisecond)
	avail := 100.0
	if total > 0 {
		avail = math.Round(((float64(total-failed)/float64(total))*100.0)*10) / 10
	}

	return &Result{
		TotalRequests:       total,
		FailedRequests:      failed,
		DowntimeMs:          downtimeMs,
		FirstFailureTime:    first,
		LastFailureTime:     last,
		AvailabilityPercent: avail,
	}
}
