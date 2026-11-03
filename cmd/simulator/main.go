package main

import (
	"flag"
	"fmt"
	"os"

	"astra/pkg/simulator"
)

func main() {
	seed := flag.Int64("seed", 42, "Deterministic pseudo-random generator seed")
	nodes := flag.Int("nodes", 6, "Number of cluster nodes to simulate")
	workloads := flag.Int("workloads", 40, "Total number of workloads to generate")
	flag.Parse()

	cfg := simulator.DefaultScenario(*seed)
	cfg.NodeCount = *nodes
	cfg.WorkloadCount = *workloads

	fmt.Printf("========================================================================\n")
	fmt.Printf("  ASTRA — Deterministic Workload Interference Simulator v0.1.0\n")
	fmt.Printf("========================================================================\n")
	fmt.Printf("Seed: %d | Nodes: %d | Workloads: %d\n\n", cfg.Seed, cfg.NodeCount, cfg.WorkloadCount)

	sim := simulator.NewClusterSimulator(cfg)

	// Execute Baseline
	baselineReport := sim.Run(simulator.PolicyBaseline)

	// Execute ASTRA
	astraReport := sim.Run(simulator.PolicyAstra)

	// Print Comparison Table
	fmt.Printf("%-30s | %-15s | %-15s\n", "Metric", "Baseline (Static)", "ASTRA (Dynamic)")
	fmt.Printf("------------------------------------------------------------------------\n")
	fmt.Printf("%-30s | %-15d | %-15d\n", "Placed Workloads", baselineReport.PlacedWorkloads, astraReport.PlacedWorkloads)
	fmt.Printf("%-30s | %-15d | %-15d\n", "Rejected Workloads", baselineReport.RejectedWorkloads, astraReport.RejectedWorkloads)
	fmt.Printf("%-30s | %-15d | %-15d\n", "Latency-Sensitive Tasks", baselineReport.LatencySensWorkloads, astraReport.LatencySensWorkloads)
	fmt.Printf("%-30s | %-15d | %-15d\n", "SLO Violations", baselineReport.SLOViolations, astraReport.SLOViolations)
	fmt.Printf("%-30s | %-14.1f%% | %-14.1f%%\n", "SLO Violation Rate", baselineReport.SLOViolationRate, astraReport.SLOViolationRate)
	fmt.Printf("%-30s | %-14.1f%% | %-14.1f%%\n", "Avg CPU Allocation", baselineReport.AvgNodeCPUUtilization, astraReport.AvgNodeCPUUtilization)
	fmt.Printf("%-30s | %-14.1f%% | %-14.1f%%\n", "Avg Memory Allocation", baselineReport.AvgNodeMemUtilization, astraReport.AvgNodeMemUtilization)
	fmt.Printf("%-30s | %-12.1f us | %-12.1f us\n", "Avg Runqueue Latency", baselineReport.AvgRunqueueLatencyUs, astraReport.AvgRunqueueLatencyUs)
	fmt.Printf("%-30s | %-12.1f us | %-12.1f us\n", "Max Runqueue Latency", baselineReport.MaxRunqueueLatencyUs, astraReport.MaxRunqueueLatencyUs)
	fmt.Printf("------------------------------------------------------------------------\n")

	reduction := 0.0
	if baselineReport.SLOViolations > 0 {
		reduction = (float64(baselineReport.SLOViolations-astraReport.SLOViolations) / float64(baselineReport.SLOViolations)) * 100.0
	}
	fmt.Printf("Outcome: ASTRA reduced SLO violations by %.1f%%\n", reduction)

	if astraReport.SLOViolations > baselineReport.SLOViolations {
		fmt.Fprintf(os.Stderr, "FAILURE: ASTRA incurred higher SLO violations than baseline\n")
		os.Exit(1)
	}
}
