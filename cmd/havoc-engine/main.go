package main

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"havoc-engine/internal/config"
	"havoc-engine/internal/k8sclient"
	"havoc-engine/internal/metrics"
	"havoc-engine/internal/safety"
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
			deletedPod, err := runner.ExecuteKillPod(context.Background(), ns, selector)
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

	rootCmd.AddCommand(killPodCmd, injectLatencyCmd, spikeCPUCmd, serveCmd)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
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
