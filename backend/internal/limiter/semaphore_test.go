package limiter

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func newTestClient(t *testing.T) *redis.Client {
	t.Helper()

	addr := os.Getenv("MUARA_E2E_REDIS")
	if addr == "" {
		t.Skip("set MUARA_E2E_REDIS=<host:port> to run against a live redis")
	}

	client := redis.NewClient(&redis.Options{Addr: addr, DB: 15})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		t.Fatalf("redis at %s is not reachable: %v", addr, err)
	}

	t.Cleanup(func() { _ = client.Close() })
	return client
}

func newTestSemaphore(t *testing.T, client *redis.Client, key string, budget int, lease, deadline time.Duration) *Semaphore {
	t.Helper()
	return NewSemaphore(client, key, budget, lease, deadline, nil)
}

func TestAcquireWithoutRedisRunsUnlimited(t *testing.T) {
	// A dev process with no shared Redis must still execute queries.
	s := NewSemaphore(nil, "muara:duckdb", 1, time.Second, time.Second, nil)

	release, err := s.Acquire(context.Background())
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	release()
	release() // idempotent
}

func TestSemaphoreHoldsBudgetAndReleases(t *testing.T) {
	client := newTestClient(t)
	key := testKey(t, client, "single")
	s := newTestSemaphore(t, client, key, 1, time.Minute, 30*time.Second)

	ctx := context.Background()
	first, err := s.Acquire(ctx)
	if err != nil {
		t.Fatalf("first acquire: %v", err)
	}

	// Budget 1 is now taken, so a short deadline must time out rather than steal.
	quick := newTestSemaphore(t, client, key, 1, time.Minute, 150*time.Millisecond)
	if _, err := quick.Acquire(ctx); !errors.Is(err, ErrNoSlot) {
		t.Errorf("second acquire error = %v, want ErrNoSlot", err)
	}

	first()
	second, err := quick.Acquire(ctx)
	if err != nil {
		t.Fatalf("acquire after release: %v", err)
	}
	second()
}

// The real guarantee under load: no matter how many callers arrive at once, the
// number running together never exceeds the budget, and everyone still finishes.
func TestSemaphoreNeverExceedsBudgetUnderLoad(t *testing.T) {
	client := newTestClient(t)
	key := testKey(t, client, "load")

	const (
		budget   = 2
		callers  = 24
		holdTime = 40 * time.Millisecond
	)

	// Every caller goes through its own Semaphore, the way separate API and
	// worker replicas would; only Redis is shared.
	sems := make([]*Semaphore, callers)
	for i := range sems {
		sems[i] = newTestSemaphore(t, client, key, budget, time.Minute, 30*time.Second)
	}

	var running, peak, done atomic.Int64
	var wg sync.WaitGroup
	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			release, err := sems[i].Acquire(context.Background())
			if err != nil {
				t.Errorf("caller %d: %v", i, err)
				return
			}
			n := running.Add(1)
			for {
				old := peak.Load()
				if n <= old || peak.CompareAndSwap(old, n) {
					break
				}
			}
			time.Sleep(holdTime)
			running.Add(-1)
			release()
			done.Add(1)
		}(i)
	}
	wg.Wait()

	if int(done.Load()) != callers {
		t.Errorf("%d callers finished, want %d", done.Load(), callers)
	}
	if peak.Load() > budget {
		t.Errorf("peak concurrency = %d, above the budget of %d", peak.Load(), budget)
	}
	if peak.Load() < 2 {
		t.Errorf("peak concurrency = %d, the budget was never used under load", peak.Load())
	}

	// Every holder releases its member, so the set must be empty again.
	if n, err := client.ZCard(context.Background(), key).Result(); err != nil {
		t.Fatalf("zcard: %v", err)
	} else if n != 0 {
		t.Errorf("%d slots left in %s after every holder released", n, key)
	}
}

// A killed process never runs its release, so its lease has to expire and free
// the budget on its own.
func TestSemaphoreReclaimsAbandonedLease(t *testing.T) {
	client := newTestClient(t)
	key := testKey(t, client, "abandoned")

	lease := 2 * time.Second
	stolen := time.Now().Add(-3 * lease).UnixMilli()
	if err := client.ZAdd(context.Background(), key, redis.Z{
		Score:  float64(stolen),
		Member: "process-that-crashed",
	}).Err(); err != nil {
		t.Fatalf("seed stale member: %v", err)
	}

	s := newTestSemaphore(t, client, key, 1, lease, time.Second)
	release, err := s.Acquire(context.Background())
	if err != nil {
		t.Fatalf("acquire over an abandoned lease: %v", err)
	}
	defer release()

	members, err := client.ZRange(context.Background(), key, 0, -1).Result()
	if err != nil {
		t.Fatalf("zrange: %v", err)
	}
	for _, m := range members {
		if m == "process-that-crashed" {
			t.Error("the stale holder was not pruned")
		}
	}
}

func TestAcquireHonoursCallerCancellation(t *testing.T) {
	client := newTestClient(t)
	key := testKey(t, client, "cancel")

	holder := newTestSemaphore(t, client, key, 1, time.Minute, time.Minute)
	release, err := holder.Acquire(context.Background())
	if err != nil {
		t.Fatalf("hold the budget: %v", err)
	}
	defer release()

	ctx, cancel := context.WithCancel(context.Background())
	waiter := newTestSemaphore(t, client, key, 1, time.Minute, time.Minute)

	errc := make(chan error, 1)
	go func() {
		_, err := waiter.Acquire(ctx)
		errc <- err
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case err := <-errc:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("error = %v, want context.Canceled", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("cancelled caller never returned")
	}
}

// Renewal keeps a long holder alive instead of being pruned mid-query.
func TestSemaphoreRenewsLongHolds(t *testing.T) {
	client := newTestClient(t)
	key := testKey(t, client, "renew")

	lease := 900 * time.Millisecond
	s := NewSemaphore(client, key, 1, lease, 10*time.Second, nil)
	s.renew = 300 * time.Millisecond

	release, err := s.Acquire(context.Background())
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	defer release()

	// Hold past several lease windows: without renewal the member would age out
	// and a second holder could join.
	time.Sleep(2 * lease)

	quick := NewSemaphore(client, key, 1, lease, 200*time.Millisecond, nil)
	if _, err := quick.Acquire(context.Background()); !errors.Is(err, ErrNoSlot) {
		t.Errorf("renewing holder lost its slot: err = %v", err)
	}
}

func testKey(t *testing.T, client *redis.Client, suffix string) string {
	t.Helper()
	key := fmt.Sprintf("muara-test:%s:%d", suffix, time.Now().UnixNano())
	t.Cleanup(func() { _ = client.Del(context.Background(), key) })
	return key
}
