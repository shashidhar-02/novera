// Package coordination implements high-availability leader election inspired by Kubernetes Leases.
package coordination

import (
	"context"
	"errors"
	"sync"
	"time"
)

var (
	ErrNotLeader     = errors.New("instance is not the current leader")
	ErrLeaseHeld     = errors.New("lease is currently held by another active leader")
	ErrEmptyLeaderID = errors.New("candidate leader ID must not be empty")
)

// LeaseStore abstracts persistence for the distributed coordination lease.
type LeaseStore interface {
	Get(ctx context.Context) (*Lease, error)
	AcquireOrRenew(ctx context.Context, lease *Lease) error
}

// Lease represents a distributed coordination lease state.
type Lease struct {
	HolderIdentity       string    `json:"holder_identity"`
	LeaseDurationSeconds int64     `json:"lease_duration_seconds"`
	AcquireTime          time.Time `json:"acquire_time"`
	RenewTime            time.Time `json:"renew_time"`
	LeaderTransitions    uint64    `json:"leader_transitions"`
}

// IsExpired returns true if the lease has lapsed past its renew time + duration.
func (l *Lease) IsExpired(now time.Time) bool {
	if l == nil || l.HolderIdentity == "" {
		return true
	}
	expiresAt := l.RenewTime.Add(time.Duration(l.LeaseDurationSeconds) * time.Second)
	return now.After(expiresAt)
}

// MemoryLeaseStore provides an in-memory, thread-safe implementation of LeaseStore.
type MemoryLeaseStore struct {
	mu      sync.Mutex
	current *Lease
}

// NewMemoryLeaseStore initializes an in-memory lease store.
func NewMemoryLeaseStore() *MemoryLeaseStore {
	return &MemoryLeaseStore{}
}

func (s *MemoryLeaseStore) Get(ctx context.Context) (*Lease, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.current == nil {
		return nil, nil
	}
	copyLease := *s.current
	return &copyLease, nil
}

func (s *MemoryLeaseStore) AcquireOrRenew(ctx context.Context, incoming *Lease) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := incoming.RenewTime
	if s.current == nil || s.current.IsExpired(now) {
		// Acquire fresh lease
		if s.current != nil && s.current.HolderIdentity != incoming.HolderIdentity {
			incoming.LeaderTransitions = s.current.LeaderTransitions + 1
		}
		s.current = incoming
		return nil
	}

	// Renewal by existing holder
	if s.current.HolderIdentity == incoming.HolderIdentity {
		incoming.LeaderTransitions = s.current.LeaderTransitions
		incoming.AcquireTime = s.current.AcquireTime
		s.current = incoming
		return nil
	}

	return ErrLeaseHeld
}

// LeaderCallbacks defines events triggered during leadership transitions.
type LeaderCallbacks struct {
	OnStartedLeading func(ctx context.Context)
	OnStoppedLeading func()
	OnNewLeader      func(identity string)
}

// LeaderElector manages background lease acquisition, heartbeating, and transitions.
type LeaderElector struct {
	candidateID string
	durationSec int64
	renewPeriod time.Duration
	store       LeaseStore
	callbacks   LeaderCallbacks
	isLeader    bool
	mu          sync.RWMutex
}

// Config configures the leader election engine.
type Config struct {
	CandidateID          string
	LeaseDurationSeconds int64
	RenewInterval        time.Duration
	Store                LeaseStore
	Callbacks            LeaderCallbacks
}

// NewLeaderElector initializes a leader elector.
func NewLeaderElector(cfg Config) (*LeaderElector, error) {
	if cfg.CandidateID == "" {
		return nil, ErrEmptyLeaderID
	}
	if cfg.LeaseDurationSeconds <= 0 {
		cfg.LeaseDurationSeconds = 15 // Standard Kubernetes default
	}
	if cfg.RenewInterval <= 0 {
		cfg.RenewInterval = 3 * time.Second
	}
	if cfg.Store == nil {
		cfg.Store = NewMemoryLeaseStore()
	}

	return &LeaderElector{
		candidateID: cfg.CandidateID,
		durationSec: cfg.LeaseDurationSeconds,
		renewPeriod: cfg.RenewInterval,
		store:       cfg.Store,
		callbacks:   cfg.Callbacks,
	}, nil
}

// IsLeader returns true if this instance currently holds the lease. Thread-safe.
func (le *LeaderElector) IsLeader() bool {
	le.mu.RLock()
	defer le.mu.RUnlock()
	return le.isLeader
}

// Run executes the continuous leader election loop until context is canceled.
func (le *LeaderElector) Run(ctx context.Context) {
	ticker := time.NewTicker(le.renewPeriod)
	defer ticker.Stop()

	// Initial immediate attempt
	le.tryAcquireOrRenew(ctx)

	for {
		select {
		case <-ctx.Done():
			le.mu.Lock()
			if le.isLeader {
				le.isLeader = false
				if le.callbacks.OnStoppedLeading != nil {
					le.callbacks.OnStoppedLeading()
				}
			}
			le.mu.Unlock()
			return
		case <-ticker.C:
			le.tryAcquireOrRenew(ctx)
		}
	}
}

func (le *LeaderElector) tryAcquireOrRenew(ctx context.Context) {
	now := time.Now().UTC()

	existing, err := le.store.Get(ctx)
	if err != nil {
		le.handleFailure()
		return
	}

	if existing != nil && !existing.IsExpired(now) && existing.HolderIdentity != le.candidateID {
		// Another node is healthy leader
		le.handleFailure()
		if le.callbacks.OnNewLeader != nil {
			le.callbacks.OnNewLeader(existing.HolderIdentity)
		}
		return
	}

	// Attempt acquire or renew
	newLease := &Lease{
		HolderIdentity:       le.candidateID,
		LeaseDurationSeconds: le.durationSec,
		AcquireTime:          now,
		RenewTime:            now,
	}

	if err := le.store.AcquireOrRenew(ctx, newLease); err != nil {
		le.handleFailure()
		return
	}

	// Successfully acquired or renewed
	le.mu.Lock()
	wasLeader := le.isLeader
	le.isLeader = true
	le.mu.Unlock()

	if !wasLeader && le.callbacks.OnStartedLeading != nil {
		le.callbacks.OnStartedLeading(ctx)
	}
}

func (le *LeaderElector) handleFailure() {
	le.mu.Lock()
	wasLeader := le.isLeader
	le.isLeader = false
	le.mu.Unlock()

	if wasLeader && le.callbacks.OnStoppedLeading != nil {
		le.callbacks.OnStoppedLeading()
	}
}
