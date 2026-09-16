package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"havoc-engine/internal/config"
	"havoc-engine/internal/experimentdef"
	"havoc-engine/internal/k8sclient"
	"havoc-engine/internal/metrics"
	"havoc-engine/internal/postmortem"
	"havoc-engine/internal/safety"
	"havoc-engine/internal/scheduler"
	"havoc-engine/internal/scoring"
	"havoc-engine/internal/storage"
)

var (
	configPath string
	namespace  string
	dryRun     bool
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "havoc-engine",
		Short: "Havoc Engine is a Kubernetes chaos engineering CLI tool",
	}

	// Start metrics server in the background
	metrics.StartMetricsServer(9090)
	
	// Initialize Prometheus metrics checker with a base simulated error rate
	metricsChecker := metrics.NewPrometheusMetricsChecker(2.0)

	rootCmd.PersistentFlags().StringVar(&configPath, "config", "config.yaml", "Path to config YAML file")
	rootCmd.PersistentFlags().StringVarP(&namespace, "namespace", "n", "", "Kubernetes namespace (defaults to config default_namespace)")
	rootCmd.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "Simulate execution without making actual changes")

	// kill-pod command
	var selector string
	killPodCmd := &cobra.Command{
		Use:   "kill-pod",
		Short: "Delete a random pod matching the selector",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadConfig(configPath)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			ns := resolveNamespace(namespace, cfg.DefaultNamespace)
			client, err := k8sclient.NewClient(cfg.Kubeconfig)
			if err != nil && !dryRun {
				return fmt.Errorf("failed to create k8s client: %w", err)
			}

			runner := safety.NewSafetyRunner(client, cfg.MaxBlastRadiusPercent, cfg.AbortOnErrorRatePercent, dryRun, metricsChecker)
			deletedPod, recoveryTimeMs, err := runner.ExecuteKillPod(context.Background(), ns, selector)
			
			// Scoring & Storage logic
			if !dryRun {
				recordExperiment(cfg, "kill-pod", ns, recoveryTimeMs, err)
			}

			if err != nil {
				return err
			}

			if !dryRun {
				fmt.Printf("Successfully killed pod %q in namespace %q\n", deletedPod, ns)
			}
			return nil
		},
	}
	killPodCmd.Flags().StringVarP(&selector, "selector", "s", "", "Label selector (e.g. app=demo)")
	_ = killPodCmd.MarkFlagRequired("selector")

	// inject-latency command
	var (
		podName string
		delayMs int
	)
	injectLatencyCmd := &cobra.Command{
		Use:   "inject-latency",
		Short: "Inject network delay into a target pod using tc/netem via an ephemeral container",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadConfig(configPath)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			ns := resolveNamespace(namespace, cfg.DefaultNamespace)
			client, err := k8sclient.NewClient(cfg.Kubeconfig)
			if err != nil && !dryRun {
				return fmt.Errorf("failed to create k8s client: %w", err)
			}

			runner := safety.NewSafetyRunner(client, cfg.MaxBlastRadiusPercent, cfg.AbortOnErrorRatePercent, dryRun, metricsChecker)
			err = runner.ExecuteInjectLatency(context.Background(), ns, podName, delayMs)
			
			// Scoring & Storage logic
			if !dryRun {
				recordExperiment(cfg, "inject-latency", ns, nil, err)
			}

			if err != nil {
				return err
			}

			if !dryRun {
				fmt.Printf("Successfully injected %dms network delay into pod %q in namespace %q\n", delayMs, podName, ns)
			}
			return nil
		},
	}
	injectLatencyCmd.Flags().StringVarP(&podName, "pod", "p", "", "Target pod name")
	injectLatencyCmd.Flags().IntVarP(&delayMs, "delay", "d", 0, "Delay duration in milliseconds")
	_ = injectLatencyCmd.MarkFlagRequired("pod")
	_ = injectLatencyCmd.MarkFlagRequired("delay")

	// spike-cpu command
	var durationSec int
	spikeCPUCmd := &cobra.Command{
		Use:   "spike-cpu",
		Short: "Stresses CPU inside a target pod using a stress-ng ephemeral container",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadConfig(configPath)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			ns := resolveNamespace(namespace, cfg.DefaultNamespace)
			client, err := k8sclient.NewClient(cfg.Kubeconfig)
			if err != nil && !dryRun {
				return fmt.Errorf("failed to create k8s client: %w", err)
			}

			runner := safety.NewSafetyRunner(client, cfg.MaxBlastRadiusPercent, cfg.AbortOnErrorRatePercent, dryRun, metricsChecker)
			err = runner.ExecuteSpikeCPU(context.Background(), ns, podName, durationSec)
			
			// Scoring & Storage logic
			if !dryRun {
				recordExperiment(cfg, "spike-cpu", ns, nil, err)
			}

			if err != nil {
				return err
			}

			if !dryRun {
				fmt.Printf("Successfully injected CPU stress (%ds) into pod %q in namespace %q\n", durationSec, podName, ns)
			}
			return nil
		},
	}
	spikeCPUCmd.Flags().StringVarP(&podName, "pod", "p", "", "Target pod name")
	spikeCPUCmd.Flags().IntVarP(&durationSec, "duration", "t", 0, "Stress duration in seconds")
	_ = spikeCPUCmd.MarkFlagRequired("pod")
	_ = spikeCPUCmd.MarkFlagRequired("duration")

	// serve command
	serveCmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the engine in server mode (keeps metrics endpoint alive)",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("Havoc Engine is running in server mode. Metrics available at :9090/metrics")
			// Block indefinitely to keep the server alive
			select {}
		},
	}

	// run command for Chaos-as-Code
	var file string
	runCmd := &cobra.Command{
		Use:   "run",
		Short: "Run an experiment from a YAML definition file",
		RunE: func(cmd *cobra.Command, args []string) error {
			exp, err := experimentdef.LoadExperiment(file)
			if err != nil {
				return err
			}

			if err := exp.Validate(); err != nil {
				return fmt.Errorf("experiment validation failed: %w", err)
			}

			cfg, err := config.LoadConfig(configPath)
			if err != nil {
				return fmt.Errorf("failed to load global config: %w", err)
			}

			// Override global safety limits if specified in the experiment
			maxBlast := cfg.MaxBlastRadiusPercent
			if exp.Safety.MaxBlastRadiusPercent > 0 {
				maxBlast = exp.Safety.MaxBlastRadiusPercent
			}

			abortThreshold := cfg.AbortOnErrorRatePercent
			if exp.Safety.AbortOnErrorRatePercent > 0 {
				abortThreshold = exp.Safety.AbortOnErrorRatePercent
			}

			client, err := k8sclient.NewClient(cfg.Kubeconfig)
			if err != nil && !dryRun {
				return fmt.Errorf("failed to create k8s client: %w", err)
			}

			runner := safety.NewSafetyRunner(client, maxBlast, abortThreshold, dryRun, metricsChecker)

			if dryRun {
				fmt.Printf("[DRY-RUN] Executing experiment %q (%s)\n", exp.Name, exp.Action)
			}

			var expErr error
			var recoveryTimeMs *int
			var deletedPod string

			switch exp.Action {
			case experimentdef.ActionKillPod:
				deletedPod, recoveryTimeMs, expErr = runner.ExecuteKillPod(context.Background(), exp.Target.Namespace, exp.Target.LabelSelector)
				if expErr == nil && !dryRun {
					fmt.Printf("Successfully killed pod %q for experiment %q\n", deletedPod, exp.Name)
				}
			case experimentdef.ActionInjectLatency:
				expErr = runner.ExecuteInjectLatency(context.Background(), exp.Target.Namespace, exp.Target.PodName, exp.Params.DelayMs)
				if expErr == nil && !dryRun {
					fmt.Printf("Successfully injected %dms latency into pod %q for experiment %q\n", exp.Params.DelayMs, exp.Target.PodName, exp.Name)
				}
			case experimentdef.ActionSpikeCPU:
				expErr = runner.ExecuteSpikeCPU(context.Background(), exp.Target.Namespace, exp.Target.PodName, exp.Params.DurationSec)
				if expErr == nil && !dryRun {
					fmt.Printf("Successfully injected CPU stress (%ds) into pod %q for experiment %q\n", exp.Params.DurationSec, exp.Target.PodName, exp.Name)
				}
			}

			if !dryRun {
				recordExperiment(cfg, exp.Action, exp.Target.Namespace, recoveryTimeMs, expErr)
			}

			if expErr != nil {
				return expErr
			}
			return nil
		},
	}
	runCmd.Flags().StringVarP(&file, "file", "f", "", "Path to experiment YAML file")
	_ = runCmd.MarkFlagRequired("file")

	// history command
	historyCmd := &cobra.Command{
		Use:   "history",
		Short: "View recent chaos experiment history and resilience scores",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadConfig(configPath)
			if err != nil {
				return err
			}
			repo, err := storage.NewPostgresRepository(cfg.DatabaseDSN)
			if err != nil {
				return err
			}
			defer repo.Close()

			results, err := repo.GetRecentExperiments(context.Background(), 10)
			if err != nil {
				return err
			}

			fmt.Println("Recent Experiments:")
			fmt.Printf("%-36s | %-15s | %-10s | %-12s | %-10s | %-17s | %-5s\n", "ID", "Type", "Namespace", "Recovery(ms)", "Error(%)", "Safety Intervened", "Score")
			fmt.Println(strings.Repeat("-", 120))
			for _, r := range results {
				rec := "N/A"
				if r.RecoveryTimeMs != nil {
					rec = fmt.Sprintf("%d", *r.RecoveryTimeMs)
				}
				errRate := "N/A"
				if r.ErrorRatePercent != nil {
					errRate = fmt.Sprintf("%.2f", *r.ErrorRatePercent)
				}
				fmt.Printf("%-36s | %-15s | %-10s | %-12s | %-10s | %-17t | %-5d\n",
					r.ID, r.ExperimentType, r.TargetNamespace, rec, errRate, r.SafetyIntervened, r.ResilienceScore)
			}
			return nil
		},
	}

	// score command
	scoreCmd := &cobra.Command{
		Use:   "score",
		Short: "Calculate and print the current average resilience score based on recent runs",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadConfig(configPath)
			if err != nil {
				return err
			}
			repo, err := storage.NewPostgresRepository(cfg.DatabaseDSN)
			if err != nil {
				return err
			}
			defer repo.Close()

			scores, err := repo.GetScoreHistory(context.Background(), 10)
			if err != nil {
				return err
			}

			if len(scores) == 0 {
				fmt.Println("No recent experiments found to calculate score.")
				return nil
			}

			sum := 0
			for _, s := range scores {
				sum += s
			}
			avg := float64(sum) / float64(len(scores))

			fmt.Printf("Average Resilience Score (last %d runs): %.1f / 100\n", len(scores), avg)
			
			if len(scores) >= 2 {
				last := scores[0]
				if float64(last) > avg {
					fmt.Println("Trend: 📈 Improving")
				} else if float64(last) < avg {
					fmt.Println("Trend: 📉 Declining")
				} else {
					fmt.Println("Trend: ➡️ Stable")
				}
			}
			
			return nil
		},
	}

	// report command
	var reportId string
	var latest bool
	reportCmd := &cobra.Command{
		Use:   "report",
		Short: "View an AI-generated postmortem report for a past experiment",
		RunE: func(cmd *cobra.Command, args []string) error {
			if !latest && reportId == "" {
				return fmt.Errorf("must specify either --id or --latest")
			}
			
			cfg, err := config.LoadConfig(configPath)
			if err != nil {
				return err
			}
			repo, err := storage.NewPostgresRepository(cfg.DatabaseDSN)
			if err != nil {
				return err
			}
			defer repo.Close()

			var exp storage.ExperimentResult
			
			// We can fetch the most recent experiments and pick the first or match the ID
			results, err := repo.GetRecentExperiments(context.Background(), 100)
			if err != nil {
				return err
			}
			
			if len(results) == 0 {
				fmt.Println("No experiments found.")
				return nil
			}
			
			found := false
			if latest {
				exp = results[0]
				found = true
			} else {
				for _, r := range results {
					if r.ID.String() == reportId {
						exp = r
						found = true
						break
					}
				}
			}
			
			if !found {
				return fmt.Errorf("experiment not found")
			}
			
			fmt.Printf("=== Postmortem Report for Experiment %s ===\n", exp.ID)
			fmt.Printf("Type: %s\n", exp.ExperimentType)
			fmt.Printf("Date: %s\n\n", exp.CreatedAt.Format(time.RFC1123))
			
			if exp.PostmortemReport != nil && *exp.PostmortemReport != "" {
				fmt.Println(*exp.PostmortemReport)
			} else {
				fmt.Println("No report available.")
			}
			
			return nil
		},
	}
	reportCmd.Flags().StringVar(&reportId, "id", "", "Experiment ID")
	reportCmd.Flags().BoolVar(&latest, "latest", false, "Get report for the most recent experiment")

	// cron command group
	cronCmd := &cobra.Command{
		Use:   "cron",
		Short: "Manage the chaos scheduler",
	}

	cronStartCmd := &cobra.Command{
		Use:   "start",
		Short: "Start the chaos scheduler",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadConfig(configPath)
			if err != nil {
				return err
			}
			repo, err := storage.NewPostgresRepository(cfg.DatabaseDSN)
			if err != nil {
				return err
			}
			defer repo.Close()

			_ = repo.RunMigrations(context.Background())

			experimentsDir := "experiments"
			sched := scheduler.NewScheduler(cfg, repo, scheduler.RealClock{}, experimentsDir, metricsChecker)
			sched.Run(context.Background())
			return nil
		},
	}

	cronPauseCmd := &cobra.Command{
		Use:   "pause",
		Short: "Pause the chaos scheduler",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadConfig(configPath)
			if err != nil {
				return err
			}
			repo, err := storage.NewPostgresRepository(cfg.DatabaseDSN)
			if err != nil {
				return err
			}
			defer repo.Close()

			if err := repo.SetSchedulerState(context.Background(), true); err != nil {
				return fmt.Errorf("failed to pause scheduler: %w", err)
			}
			fmt.Println("Chaos scheduler paused.")
			return nil
		},
	}

	cronResumeCmd := &cobra.Command{
		Use:   "resume",
		Short: "Resume the chaos scheduler",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadConfig(configPath)
			if err != nil {
				return err
			}
			repo, err := storage.NewPostgresRepository(cfg.DatabaseDSN)
			if err != nil {
				return err
			}
			defer repo.Close()

			if err := repo.SetSchedulerState(context.Background(), false); err != nil {
				return fmt.Errorf("failed to resume scheduler: %w", err)
			}
			fmt.Println("Chaos scheduler resumed.")
			return nil
		},
	}

	cronStatusCmd := &cobra.Command{
		Use:   "status",
		Short: "View the status and recent logs of the chaos scheduler",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadConfig(configPath)
			if err != nil {
				return err
			}
			repo, err := storage.NewPostgresRepository(cfg.DatabaseDSN)
			if err != nil {
				return err
			}
			defer repo.Close()

			paused, err := repo.GetSchedulerState(context.Background())
			if err != nil {
				return fmt.Errorf("failed to get scheduler state: %w", err)
			}

			if paused {
				fmt.Println("Scheduler Status: PAUSED ⏸️")
			} else {
				fmt.Println("Scheduler Status: RUNNING ▶️")
			}

			runs, err := repo.GetRecentScheduledRuns(context.Background(), 10)
			if err != nil {
				return fmt.Errorf("failed to get recent scheduled runs: %w", err)
			}

			fmt.Println("\nRecent Scheduled Runs:")
			fmt.Printf("%-25s | %-30s | %-20s\n", "Time", "Experiment", "Outcome")
			fmt.Println(strings.Repeat("-", 80))
			for _, r := range runs {
				fmt.Printf("%-25s | %-30s | %-20s\n", r.Timestamp.Format(time.RFC3339), r.ExperimentFile, r.Outcome)
			}
			return nil
		},
	}

	cronCmd.AddCommand(cronStartCmd, cronPauseCmd, cronResumeCmd, cronStatusCmd)

	rootCmd.AddCommand(killPodCmd, injectLatencyCmd, spikeCPUCmd, serveCmd, runCmd, historyCmd, scoreCmd, reportCmd, cronCmd)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func recordExperiment(cfg *config.Config, expType, namespace string, recoveryTimeMs *int, expErr error) {
	// Initialize repository
	repo, err := storage.NewPostgresRepository(cfg.DatabaseDSN)
	if err != nil {
		fmt.Printf("Warning: failed to connect to database for scoring: %v\n", err)
		return
	}
	defer repo.Close()

	// Ensure migrations are run (lazy init for simplicity in CLI)
	_ = repo.RunMigrations(context.Background())

	// Determine safety intervention
	safetyIntervened := false
	if expErr != nil && strings.Contains(expErr.Error(), "auto-abort triggered") {
		safetyIntervened = true
	}

	// For error rate, we'd ideally query the prometheus backend, but since this is CLI-driven,
	// let's grab the current error rate from our dummy metrics checker.
	checker := metrics.NewPrometheusMetricsChecker(2.0)
	var errRate *float64
	if rate, err := checker.GetErrorRate(context.Background(), namespace); err == nil {
		errRate = &rate
	}

	var errorRateVal float64
	if errRate != nil {
		errorRateVal = *errRate
	}

	score := scoring.CalculateResilienceScore(recoveryTimeMs, errorRateVal, safetyIntervened, cfg.Scoring)

	// Generate Postmortem Report
	var reportTextPtr *string
	if cfg.AnthropicAPIKey != "" {
		fmt.Println("Generating AI postmortem report...")
		gen, err := postmortem.NewClaudeGenerator(cfg.AnthropicAPIKey)
		if err != nil {
			fmt.Printf("Warning: failed to initialize postmortem generator: %v\n", err)
		} else {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			
			report, err := gen.GenerateReport(ctx, postmortem.ExperimentData{
				ExperimentType:   expType,
				TargetNamespace:  namespace,
				RecoveryTimeMs:   recoveryTimeMs,
				ErrorRatePercent: errRate,
				SafetyIntervened: safetyIntervened,
				ResilienceScore:  score,
			})
			if err != nil {
				fmt.Printf("Warning: failed to generate postmortem report (API Error): %v\n", err)
			} else {
				reportTextPtr = &report
				fmt.Println("Successfully generated AI postmortem report.")
			}
		}
	}

	res := &storage.ExperimentResult{
		ID:               uuid.New(),
		ExperimentType:   expType,
		TargetNamespace:  namespace,
		RecoveryTimeMs:   recoveryTimeMs,
		ErrorRatePercent: errRate,
		SafetyIntervened: safetyIntervened,
		ResilienceScore:  score,
		PostmortemReport: reportTextPtr,
		CreatedAt:        time.Now(),
	}

	if err := repo.SaveExperimentResult(context.Background(), res); err != nil {
		fmt.Printf("Warning: failed to save experiment result: %v\n", err)
	} else {
		fmt.Printf("Recorded experiment result: Resilience Score %d/100\n", score)
	}
}

func resolveNamespace(flagNs, defaultNs string) string {
	if flagNs != "" {
		return flagNs
	}
	if defaultNs != "" {
		return defaultNs
	}
	return "default"
}
