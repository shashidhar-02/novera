package simulator

import (
	"testing"
)

func TestSimulator_DeterministicComparison(t *testing.T) {
	scenario := DefaultScenario(42) // Fixed seed
	sim := NewClusterSimulator(scenario)

	// 1. Run Baseline Policy
	baselineReport := sim.Run(PolicyBaseline)
	if baselineReport.PlacedWorkloads == 0 {
		t.Fatalf("expected baseline to place workloads")
	}

	// 2. Run ASTRA Policy on the exact same scenario
	astraReport := sim.Run(PolicyAstra)
	if astraReport.PlacedWorkloads == 0 {
		t.Fatalf("expected ASTRA to place workloads")
	}

	t.Logf("=== Deterministic Simulation Benchmark (Seed: %d) ===", scenario.Seed)
	t.Logf("Baseline SLO Violations: %d / %d (%.1f%%)",
		baselineReport.SLOViolations, baselineReport.LatencySensWorkloads, baselineReport.SLOViolationRate)
	t.Logf("ASTRA SLO Violations:    %d / %d (%.1f%%)",
		astraReport.SLOViolations, astraReport.LatencySensWorkloads, astraReport.SLOViolationRate)
	t.Logf("Baseline Max Runqueue:   %.1f us", baselineReport.MaxRunqueueLatencyUs)
	t.Logf("ASTRA Max Runqueue:      %.1f us", astraReport.MaxRunqueueLatencyUs)

	// Verification invariant: ASTRA's interference-aware steering should strictly reduce or match SLO violations
	if astraReport.SLOViolations > baselineReport.SLOViolations {
		t.Errorf("ASTRA had more SLO violations (%d) than baseline (%d)",
			astraReport.SLOViolations, baselineReport.SLOViolations)
	}
}
