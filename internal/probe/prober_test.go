package probe

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProber_SkippedWhenNoURL(t *testing.T) {
	t.Run("empty URL in config", func(t *testing.T) {
		p := NewProber(Config{URL: ""})
		stop := p.Start(context.Background())
		res := stop()
		assert.Nil(t, res)
	})

	t.Run("nil prober", func(t *testing.T) {
		var p *Prober
		stop := p.Start(context.Background())
		res := stop()
		assert.Nil(t, res)
	})
}

func TestProber_ControlledFailureWindow(t *testing.T) {
	var count int64

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		idx := atomic.AddInt64(&count, 1)
		// Requests 1..2 return 200 OK
		// Requests 3..6 return 500 Internal Server Error (4 failures)
		// Requests 7+ return 200 OK
		if idx >= 3 && idx <= 6 {
			http.Error(w, "chaos injected", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()

	interval := 15 * time.Millisecond
	p := NewProber(Config{
		URL:         server.URL,
		Interval:    interval,
		Timeout:     50 * time.Millisecond,
		GracePeriod: -1,
		Client:      server.Client(),
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stop := p.Start(ctx)

	// Wait until at least 8 requests are made
	require.Eventually(t, func() bool {
		return atomic.LoadInt64(&count) >= 8
	}, 1*time.Second, 10*time.Millisecond)

	res := stop()
	require.NotNil(t, res)

	assert.GreaterOrEqual(t, res.TotalRequests, 8)
	assert.Equal(t, 4, res.FailedRequests)
	// Longest streak was 4 consecutive failures
	assert.Equal(t, 4*int(interval/time.Millisecond), res.DowntimeMs)
	assert.NotNil(t, res.FirstFailureTime)
	assert.NotNil(t, res.LastFailureTime)
	assert.True(t, res.LastFailureTime.After(*res.FirstFailureTime) || res.LastFailureTime.Equal(*res.FirstFailureTime))

	expectedAvail := float64(res.TotalRequests-4) / float64(res.TotalRequests) * 100.0
	assert.InDelta(t, expectedAvail, res.AvailabilityPercent, 0.2)
}

func TestProber_AllSuccessful(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	p := NewProber(Config{
		URL:         server.URL,
		Interval:    10 * time.Millisecond,
		GracePeriod: -1,
		Client:      server.Client(),
	})

	stop := p.Start(context.Background())
	time.Sleep(35 * time.Millisecond)
	res := stop()

	require.NotNil(t, res)
	assert.GreaterOrEqual(t, res.TotalRequests, 2)
	assert.Equal(t, 0, res.FailedRequests)
	assert.Equal(t, 0, res.DowntimeMs)
	assert.Equal(t, 100.0, res.AvailabilityPercent)
	assert.Nil(t, res.FirstFailureTime)
	assert.Nil(t, res.LastFailureTime)
}

func TestProber_GracePeriod(t *testing.T) {
	var count int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(&count, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	gracePeriod := 60 * time.Millisecond
	p := NewProber(Config{
		URL:         server.URL,
		Interval:    15 * time.Millisecond,
		GracePeriod: gracePeriod,
		Client:      server.Client(),
	})

	stop := p.Start(context.Background())
	time.Sleep(20 * time.Millisecond)
	countAtStop := atomic.LoadInt64(&count)

	startStop := time.Now()
	res := stop()
	elapsed := time.Since(startStop)

	require.NotNil(t, res)
	assert.GreaterOrEqual(t, elapsed, gracePeriod-10*time.Millisecond)
	assert.Greater(t, int64(res.TotalRequests), countAtStop, "should continue probing during grace period")
}

func TestProber_TimeoutsAndNetworkErrors(t *testing.T) {
	// Prober against an unreachable port
	p := NewProber(Config{
		URL:         "http://127.0.0.1:54321/unreachable",
		Interval:    10 * time.Millisecond,
		Timeout:     20 * time.Millisecond,
		GracePeriod: -1,
	})

	stop := p.Start(context.Background())
	time.Sleep(35 * time.Millisecond)
	res := stop()

	require.NotNil(t, res)
	assert.GreaterOrEqual(t, res.TotalRequests, 2)
	assert.Equal(t, res.TotalRequests, res.FailedRequests)
	assert.Equal(t, 0.0, res.AvailabilityPercent)
	assert.NotNil(t, res.FirstFailureTime)
}

func TestProber_StopIdempotent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	p := NewProber(Config{
		URL:         server.URL,
		Interval:    10 * time.Millisecond,
		GracePeriod: -1,
		Client:      server.Client(),
	})

	stop := p.Start(context.Background())
	time.Sleep(20 * time.Millisecond)

	var wg sync.WaitGroup
	var res1, res2 *Result
	wg.Add(2)
	go func() {
		defer wg.Done()
		res1 = stop()
	}()
	go func() {
		defer wg.Done()
		res2 = stop()
	}()
	wg.Wait()

	assert.NotNil(t, res1)
	assert.NotNil(t, res2)
	assert.Equal(t, res1, res2)
}
