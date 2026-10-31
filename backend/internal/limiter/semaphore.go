// Package limiter arbitrates how many analytical executions may run at the same
// time across every process that shares one Redis.
package limiter

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"muara.bi/core/pkg/logging"
)

// ErrNoSlot means the budget stayed full until the caller's deadline.
var ErrNoSlot = errors.New("no analytical slot became available before the deadline")

// acquireScript claims a slot only after pruning leases nobody is renewing any
// more. The check, the prune and the write happen in one script, so two
// processes can never both take the last slot.
var acquireScript = redis.NewScript(`
local now = tonumber(ARGV[1])
local lease = tonumber(ARGV[2])
local member = ARGV[3]
local budget = tonumber(ARGV[4])
redis.call('ZREMRANGEBYSCORE', KEYS[1], 0, now - lease)
if redis.call('ZCARD', KEYS[1]) < budget then
  redis.call('ZADD', KEYS[1], now, member)
  return 1
end
return 0
`)

// renewScript extends a lease only while its member still exists. A zero answer
// means the holder died, was pruned, and must not silently re-claim a slot.
var renewScript = redis.NewScript(`
if redis.call('ZSCORE', KEYS[1], ARGV[2]) then
  redis.call('ZADD', KEYS[1], ARGV[1], ARGV[2])
  return 1
end
return 0
`)

// Semaphore is a counting semaphore backed by a Redis sorted set. One member
// per holder is scored with the time it last renewed, so an abandoned slot
// expires instead of draining the budget forever.
type Semaphore struct {
	client   *redis.Client
	key      string
	budget   int
	lease    time.Duration
	renew    time.Duration
	deadline time.Duration
	logger   *logging.Logger
}

// NewSemaphore builds a budget of `budget` simultaneous holders. `lease` must
// exceed the longest execution that will be guarded, otherwise a slow but live
// query can lose its slot to a newcomer. A nil client means no shared budget
// exists (single-process dev), and Acquire then hands out unlimited slots.
func NewSemaphore(client *redis.Client, key string, budget int, lease, deadline time.Duration, logger *logging.Logger) *Semaphore {
	if budget < 1 {
		budget = 1
	}
	if lease <= 0 {
		lease = 90 * time.Second
	}
	if deadline <= 0 {
		deadline = 30 * time.Second
	}
	return &Semaphore{
		client:   client,
		key:      key,
		budget:   budget,
		lease:    lease,
		renew:    max(lease/3, time.Second),
		deadline: deadline,
		logger:   logger,
	}
}

// Acquire waits for a slot and returns the release function that hands it back.
// The returned release is safe to call more than once.
func (s *Semaphore) Acquire(ctx context.Context) (func(), error) {
	if s == nil || s.client == nil {
		return func() {}, nil
	}

	ctx, cancel := context.WithTimeout(ctx, s.deadline)
	defer cancel()

	member := newMember()
	wait := 5 * time.Millisecond

	for {
		granted, err := s.tryAcquire(ctx, member)
		if err != nil {
			// Redis being unreachable must not take the whole platform down: the
			// per-process engine limits still apply, so the query runs unscheduled
			// rather than failing for a reason the user cannot act on.
			if !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled) {
				s.warn(fmt.Sprintf("analytical limiter is unavailable, running without a shared budget: %v", err))
				return func() {}, nil
			}
			return nil, noSlotError(ctx)
		}
		if granted {
			return s.hold(member), nil
		}

		select {
		case <-ctx.Done():
			return nil, noSlotError(ctx)
		case <-time.After(wait):
		}
		wait = min(wait*2, 50*time.Millisecond)
	}
}

// noSlotError keeps a caller disconnect as a cancellation instead of reporting
// the 429 that a genuinely full budget gets.
func noSlotError(ctx context.Context) error {
	if errors.Is(ctx.Err(), context.Canceled) {
		return context.Canceled
	}
	return ErrNoSlot
}

func (s *Semaphore) tryAcquire(ctx context.Context, member string) (bool, error) {
	now := time.Now()
	res, err := acquireScript.Run(ctx, s.client, []string{s.key},
		now.UnixMilli(), s.lease.Milliseconds(), member, s.budget).Int64()
	if err != nil {
		return false, err
	}
	return res == 1, nil
}

// hold keeps the lease alive until release runs. Renewal failures are logged but
// never block the caller: the work is already running and its own guardrails
// decide when to stop.
func (s *Semaphore) hold(member string) func() {
	stop := make(chan struct{})
	stopped := make(chan struct{})
	var once sync.Once

	go func() {
		defer close(stopped)
		ticker := time.NewTicker(s.renew)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				ctx, cancel := context.WithTimeout(context.Background(), s.renew)
				granted, err := renewScript.Run(ctx, s.client, []string{s.key},
					time.Now().UnixMilli(), member).Int64()
				cancel()
				if err != nil {
					s.warn(fmt.Sprintf("could not renew analytical slot: %v", err))
					continue
				}
				if granted != 1 {
					s.warn("analytical slot was lost while the query was still running")
					return
				}
			}
		}
	}()

	return func() {
		once.Do(func() {
			close(stop)
			<-stopped

			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			if err := s.client.ZRem(ctx, s.key, member).Err(); err != nil {
				s.warn(fmt.Sprintf("could not release analytical slot: %v", err))
			}
		})
	}
}

func (s *Semaphore) warn(message string) {
	if s.logger != nil {
		s.logger.Warn(message)
	}
}

func newMember() string {
	var b [12]byte
	if _, err := rand.Read(b[:]); err != nil {
		// The member only has to be unique per process and instant; a clock value
		// still cannot collide with another holder's random id.
		return fmt.Sprintf("holder-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b[:])
}
