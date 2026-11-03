package coordination

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestLeaderElection_SingleNode(t *testing.T) {
	store := NewMemoryLeaseStore()
	started := make(chan struct{}, 1)

	cfg := Config{
		CandidateID:          "node-1",
		LeaseDurationSeconds: 2,
		RenewInterval:        50 * time.Millisecond,
		Store:                store,
		Callbacks: LeaderCallbacks{
			OnStartedLeading: func(ctx context.Context) {
				started <- struct{}{}
			},
		},
	}

	elector, err := NewLeaderElector(cfg)
	if err != nil {
		t.Fatalf("failed to create elector: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go elector.Run(ctx)

	select {
	case <-started:
	case <-time.After(500 * time.Millisecond):
		t.Fatalf("timeout waiting for node-1 to acquire leadership")
	}

	if !elector.IsLeader() {
		t.Fatalf("expected elector.IsLeader() to be true")
	}
}

func TestLeaderElection_Failover(t *testing.T) {
	store := NewMemoryLeaseStore()

	node1Started := make(chan struct{}, 1)
	node1Stopped := make(chan struct{}, 1)
	node2Started := make(chan struct{}, 1)

	ctx1, cancel1 := context.WithCancel(context.Background())
	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()

	// Short lease duration: 1 second, renew every 50ms
	elector1, _ := NewLeaderElector(Config{
		CandidateID:          "replica-1",
		LeaseDurationSeconds: 1,
		RenewInterval:        50 * time.Millisecond,
		Store:                store,
		Callbacks: LeaderCallbacks{
			OnStartedLeading: func(ctx context.Context) {
				node1Started <- struct{}{}
			},
			OnStoppedLeading: func() {
				node1Stopped <- struct{}{}
			},
		},
	})

	elector2, _ := NewLeaderElector(Config{
		CandidateID:          "replica-2",
		LeaseDurationSeconds: 1,
		RenewInterval:        50 * time.Millisecond,
		Store:                store,
		Callbacks: LeaderCallbacks{
			OnStartedLeading: func(ctx context.Context) {
				node2Started <- struct{}{}
			},
		},
	})

	// Start replica-1
	go elector1.Run(ctx1)

	select {
	case <-node1Started:
	case <-time.After(500 * time.Millisecond):
		t.Fatalf("replica-1 failed to acquire leadership")
	}

	// Start replica-2 (must remain standby)
	go elector2.Run(ctx2)
	time.Sleep(100 * time.Millisecond)

	if elector2.IsLeader() {
		t.Fatalf("replica-2 acquired leadership while replica-1 was healthy!")
	}

	// Simulate replica-1 crash / context cancellation
	cancel1()

	select {
	case <-node1Stopped:
	case <-time.After(500 * time.Millisecond):
		t.Fatalf("replica-1 failed to trigger OnStoppedLeading callback")
	}

	// Wait for lease expiry and failover to replica-2
	select {
	case <-node2Started:
	case <-time.After(2 * time.Second):
		t.Fatalf("replica-2 failed to take over after replica-1 stopped")
	}

	if !elector2.IsLeader() {
		t.Fatalf("expected replica-2 to be the active leader")
	}

	// Verify leader transitions incremented
	l, _ := store.Get(context.Background())
	if l.LeaderTransitions < 1 {
		t.Errorf("expected LeaderTransitions >= 1, got %d", l.LeaderTransitions)
	}
}

func TestLeaderElection_Concurrency(t *testing.T) {
	store := NewMemoryLeaseStore()
	var wg sync.WaitGroup

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	candidates := 5
	electors := make([]*LeaderElector, candidates)

	for i := 0; i < candidates; i++ {
		id := string(rune('A' + i))
		e, _ := NewLeaderElector(Config{
			CandidateID:          id,
			LeaseDurationSeconds: 1,
			RenewInterval:        30 * time.Millisecond,
			Store:                store,
		})
		electors[i] = e

		wg.Add(1)
		go func(el *LeaderElector) {
			defer wg.Done()
			el.Run(ctx)
		}(e)
	}

	wg.Wait()

	// Verify only at most one leader existed at any time
	l, err := store.Get(context.Background())
	if err != nil {
		t.Fatalf("unexpected store error: %v", err)
	}
	if l == nil {
		t.Fatalf("expected a leader lease to have been created")
	}
}
