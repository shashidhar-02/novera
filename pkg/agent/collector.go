package agent

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"astra/pkg/model"
)

// Config configures the node telemetry collector.
type Config struct {
	NodeID          string        `json:"node_id"`
	CollectInterval time.Duration `json:"collect_interval"`
	PSIEnabled      bool          `json:"psi_enabled"`
	CgroupsEnabled  bool          `json:"cgroups_enabled"`
}

// Collector aggregates kernel-level PSI, cgroup throttling, and runqueue measurements.
type Collector struct {
	cfg       Config
	psi       PSIReader
	cgroup    CgroupReader
	seqNumber uint64
}

// NewCollector constructs a node agent telemetry collector.
func NewCollector(cfg Config, psi PSIReader, cgroup CgroupReader) *Collector {
	if cfg.CollectInterval <= 0 {
		cfg.CollectInterval = 1 * time.Second
	}
	if psi == nil {
		psi = NewOSPSIReader("")
	}
	if cgroup == nil {
		cgroup = NewOSCgroupReader("")
	}
	return &Collector{
		cfg:    cfg,
		psi:    psi,
		cgroup: cgroup,
	}
}

// CollectOnce performs a single non-blocking snapshot of host pressure and cgroup state.
func (c *Collector) CollectOnce(ctx context.Context, runqueueLatencyP99Us float64) (*model.NodeTelemetry, error) {
	if ctx != nil && ctx.Err() != nil {
		return nil, ctx.Err()
	}

	seq := atomic.AddUint64(&c.seqNumber, 1)
	now := time.Now().UTC()

	telemetry := &model.NodeTelemetry{
		NodeID:               c.cfg.NodeID,
		CollectedAt:          now,
		SequenceNumber:       seq,
		RunqueueLatencyP99Us: runqueueLatencyP99Us,
	}

	if c.cfg.PSIEnabled && c.psi != nil {
		cpuPSI, err := c.psi.ReadPressure("cpu")
		if err == nil {
			telemetry.CPUPressure = cpuPSI
		}
		memPSI, err := c.psi.ReadPressure("memory")
		if err == nil {
			telemetry.MemoryPressure = memPSI
		}
		ioPSI, err := c.psi.ReadPressure("io")
		if err == nil {
			telemetry.IOPressure = ioPSI
		}
	}

	if c.cfg.CgroupsEnabled && c.cgroup != nil {
		stat, err := c.cgroup.ReadCPUStat("")
		if err == nil {
			telemetry.CFSThrottledRatio = stat.ThrottledRatio
		}
	}

	if telemetry.NodeID == "" {
		return nil, fmt.Errorf("node ID must be specified in collector configuration")
	}

	return telemetry, nil
}

