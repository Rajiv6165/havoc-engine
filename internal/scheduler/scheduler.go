package scheduler

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"

	"havoc-engine/internal/config"
	"havoc-engine/internal/experimentdef"
	"havoc-engine/internal/k8sclient"
	"havoc-engine/internal/metrics"
	"havoc-engine/internal/safety"
	"havoc-engine/internal/storage"
)

type Scheduler struct {
	cfg            *config.Config
	repo           *storage.PostgresRepository
	clock          Clock
	experimentsDir string
	metricsChecker *metrics.PrometheusMetricsChecker
}

func NewScheduler(cfg *config.Config, repo *storage.PostgresRepository, clock Clock, experimentsDir string, mc *metrics.PrometheusMetricsChecker) *Scheduler {
	return &Scheduler{
		cfg:            cfg,
		repo:           repo,
		clock:          clock,
		experimentsDir: experimentsDir,
		metricsChecker: mc,
	}
}

func (s *Scheduler) Run(ctx context.Context) {
	log.Println("Starting Chaos Scheduler...")
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("Chaos Scheduler stopped.")
			return
		case <-ticker.C:
			s.tick(ctx)
		}
	}
}

func (s *Scheduler) tick(ctx context.Context) {
	if !s.cfg.Scheduler.Enabled {
		return
	}

	now := s.clock.Now()
	
	// 1. Check if we are inside an allowed time window
	if !s.isInAllowedWindow(now) {
		return
	}

	// 2. Check if scheduler is paused in DB
	paused, err := s.repo.GetSchedulerState(ctx)
	if err != nil {
		log.Printf("Scheduler error checking pause state: %v", err)
		return
	}
	if paused {
		// Log a skipped run because it's paused
		_ = s.repo.LogScheduledRun(ctx, "skipped_paused", "")
		return
	}

	// 3. Roll the dice to see if we should run now
	// Simplistic probability calculation based on min/max per week.
	// We run every minute. Total minutes in allowed windows per week might be large.
	// For simplicity, let's say there is a 1% chance every minute during a window to run an experiment,
	// just to simulate randomness without maintaining complex state of how many times it ran this week.
	// In a real system we'd check how many times we already ran this week in the DB.
	
	// Let's implement a simple probability:
	// A static 5% chance every tick (minute) during the allowed window.
	if rand.Float32() > 0.05 {
		return
	}

	// 4. Pick a random experiment
	expFile, err := s.pickRandomExperiment()
	if err != nil {
		log.Printf("Scheduler error picking experiment: %v", err)
		return
	}

	// 5. Run the experiment
	err = s.runExperiment(ctx, expFile)
	
	outcome := "success"
	if err != nil {
		outcome = fmt.Sprintf("error: %v", err)
		log.Printf("Scheduler experiment run failed: %v", err)
	} else {
		log.Printf("Scheduler successfully ran experiment: %s", expFile)
	}

	// 6. Log the audit record
	_ = s.repo.LogScheduledRun(ctx, outcome, expFile)
}

func (s *Scheduler) isInAllowedWindow(now time.Time) bool {
	if len(s.cfg.Scheduler.AllowedWindows) == 0 {
		return true // If no windows defined, always allowed
	}

	currentDay := now.Weekday().String()
	currentTimeStr := now.Format("15:04")

	for _, window := range s.cfg.Scheduler.AllowedWindows {
		dayMatches := false
		for _, d := range window.Days {
			if strings.EqualFold(d, currentDay) {
				dayMatches = true
				break
			}
		}

		if dayMatches {
			if currentTimeStr >= window.StartTime && currentTimeStr <= window.EndTime {
				return true
			}
		}
	}
	return false
}

func (s *Scheduler) pickRandomExperiment() (string, error) {
	files, err := os.ReadDir(s.experimentsDir)
	if err != nil {
		return "", err
	}

	var yamlFiles []string
	for _, f := range files {
		if !f.IsDir() && (strings.HasSuffix(f.Name(), ".yaml") || strings.HasSuffix(f.Name(), ".yml")) {
			yamlFiles = append(yamlFiles, f.Name())
		}
	}

	if len(yamlFiles) == 0 {
		return "", fmt.Errorf("no yaml experiments found in %s", s.experimentsDir)
	}

	idx := rand.Intn(len(yamlFiles))
	return filepath.Join(s.experimentsDir, yamlFiles[idx]), nil
}

func (s *Scheduler) runExperiment(ctx context.Context, file string) error {
	exp, err := experimentdef.LoadExperiment(file)
	if err != nil {
		return err
	}

	if err := exp.Validate(); err != nil {
		return err
	}

	maxBlast := s.cfg.MaxBlastRadiusPercent
	if exp.Safety.MaxBlastRadiusPercent > 0 {
		maxBlast = exp.Safety.MaxBlastRadiusPercent
	}

	abortThreshold := s.cfg.AbortOnErrorRatePercent
	if exp.Safety.AbortOnErrorRatePercent > 0 {
		abortThreshold = exp.Safety.AbortOnErrorRatePercent
	}

	client, err := k8sclient.NewClient(s.cfg.Kubeconfig)
	if err != nil {
		return err
	}

	runner := safety.NewSafetyRunner(client, maxBlast, abortThreshold, false, s.metricsChecker)

	var expErr error
	switch exp.Action {
	case experimentdef.ActionKillPod:
		_, _, expErr = runner.ExecuteKillPod(ctx, exp.Target.Namespace, exp.Target.LabelSelector)
	case experimentdef.ActionInjectLatency:
		expErr = runner.ExecuteInjectLatency(ctx, exp.Target.Namespace, exp.Target.PodName, exp.Params.DelayMs)
	case experimentdef.ActionSpikeCPU:
		expErr = runner.ExecuteSpikeCPU(ctx, exp.Target.Namespace, exp.Target.PodName, exp.Params.DurationSec)
	default:
		expErr = fmt.Errorf("unknown action: %s", exp.Action)
	}

	return expErr
}
