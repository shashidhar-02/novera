// Package main implements the ASTRA Scheduler control-plane daemon.
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"astra/pkg/coordination"
)

func main() {
	fmt.Println("========================================================================")
	fmt.Println("  ASTRA Scheduler Control Plane Daemon v0.1.0")
	fmt.Println("========================================================================")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle graceful shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Leader Election Setup
	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "astra-scheduler-0"
	}

	elector, err := coordination.NewLeaderElector(coordination.Config{
		CandidateID:          hostname,
		LeaseDurationSeconds: 15,
		RenewInterval:        3 * time.Second,
		Callbacks: coordination.LeaderCallbacks{
			OnStartedLeading: func(leadCtx context.Context) {
				fmt.Printf("[+] [%s] Leadership acquired! Active scheduling loop running.\n", hostname)
			},
			OnStoppedLeading: func() {
				fmt.Printf("[-] [%s] Leadership lost or released.\n", hostname)
			},
			OnNewLeader: func(identity string) {
				fmt.Printf("[*] New leader elected in cluster: %s\n", identity)
			},
		},
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize leader elector: %v\n", err)
		os.Exit(1)
	}

	go elector.Run(ctx)

	// Health and readiness HTTP server
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok\n"))
	})
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ready\n"))
	})
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("# HELP astra_scheduler_healthy ASTRA scheduler health indicator\n# TYPE astra_scheduler_healthy gauge\nastra_scheduler_healthy 1\n"))
	})

	server := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	go func() {
		fmt.Println("[*] Health and metrics server listening on :8080")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Fprintf(os.Stderr, "HTTP server error: %v\n", err)
		}
	}()

	// Wait for shutdown signal
	sig := <-sigChan
	fmt.Printf("\n[*] Received signal %s: initiating graceful shutdown...\n", sig)

	cancel() // Cancels leader elector and background workers

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	_ = server.Shutdown(shutdownCtx)

	fmt.Println("[+] ASTRA Scheduler shutdown cleanly completed.")
}

