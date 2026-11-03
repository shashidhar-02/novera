// Package main implements the ASTRA Linux Node Telemetry Agent daemon.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"astra/pkg/agent"
)

func main() {
	fmt.Println("========================================================================")
	fmt.Println("  ASTRA Linux Node Telemetry Agent Daemon v0.1.0")
	fmt.Println("========================================================================")

	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "node-01"
	}

	cfg := agent.Config{
		NodeID:          hostname,
		CollectInterval: 1 * time.Second,
		PSIEnabled:      true,
		CgroupsEnabled:  true,
	}

	collector := agent.NewCollector(cfg, nil, nil)
	fmt.Printf("[*] Initialized telemetry collector for node: %s\n", hostname)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	ticker := time.NewTicker(cfg.CollectInterval)
	defer ticker.Stop()

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				telemetry, err := collector.CollectOnce(ctx, 150.0)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Collection error: %v\n", err)
				} else {
					fmt.Printf("[telemetry] seq=%d time=%s cpu_psi=%.2f cfs_throttled=%.2f\n",
						telemetry.SequenceNumber,
						telemetry.CollectedAt.Format(time.RFC3339),
						telemetry.CPUPressure.SomeAvg10,
						telemetry.CFSThrottledRatio)
				}
			}
		}
	}()

	sig := <-sigChan
	fmt.Printf("\n[*] Received signal %s: shutting down ASTRA Node Agent...\n", sig)
	cancel()
	fmt.Println("[+] ASTRA Node Agent shutdown complete.")
}
