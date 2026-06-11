// Package ipam is OPORD's centralized CIDR allocator. It hands out /22 blocks
// from a Vault KV v2 pool using compare-and-swap (CAS) so concurrent account
// provisioning never double-allocates a range - replacing the reference's
// Jenkins "lockable resources" with an atomic, queue-friendly primitive.
//
// The pool is a single KV v2 secret whose fields are "<cidr>" => "<owner>"
// (owner empty = free). Allocation is: read (with version) to pick a free CIDR to 
// write back with cas=version; on version conflict, retry.
package ipam

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
)

var (
	// ErrPoolExhausted means no free CIDR remains in the pool.
	ErrPoolExhausted = errors.New("ipam: CIDR pool exhausted")
	// ErrCASRetriesExhausted means too many concurrent writers collided.
	ErrCASRetriesExhausted = errors.New("ipam: CAS retries exhausted")
)

const defaultMaxRetries = 6

// KV is the minimal store the allocator needs. The Vault-backed implementation
// is VaultKV; tests use an in-memory fake. Read returns the current pool map and
// its KV v2 version; CASWrite persists only if the version still matches
// (conflict=true otherwise).
type KV interface {
	Read(ctx context.Context) (pool map[string]string, version int, err error)
	CASWrite(ctx context.Context, pool map[string]string, expectedVersion int) (conflict bool, err error)
}

// Pool allocates CIDRs from a KV-backed pool.
type Pool struct {
	kv         KV
	log        *slog.Logger
	maxRetries int
}

// New builds a Pool over any KV store.
func New(kv KV, log *slog.Logger) *Pool {
	if log == nil {
		log = slog.Default()
	}
	return &Pool{kv: kv, log: log, maxRetries: defaultMaxRetries}
}

// Allocate reserves a free CIDR for owner and returns it. Idempotent: if owner
// already holds a CIDR, that same one is returned (re-runs don't leak blocks).
func (p *Pool) Allocate(ctx context.Context, owner string) (string, error) {
	if owner == "" {
		return "", fmt.Errorf("ipam: owner must be non-empty")
	}
	for attempt := 0; attempt < p.maxRetries; attempt++ {
		pool, version, err := p.kv.Read(ctx)
		if err != nil {
			return "", fmt.Errorf("ipam: read pool: %w", err)
		}
		// Idempotency: already allocated to this owner?
		if cidr := ownedBy(pool, owner); cidr != "" {
			return cidr, nil
		}
		free := firstFree(pool)
		if free == "" {
			return "", ErrPoolExhausted
		}
		pool[free] = owner
		conflict, err := p.kv.CASWrite(ctx, pool, version)
		if err != nil {
			return "", fmt.Errorf("ipam: write pool: %w", err)
		}
		if !conflict {
			p.log.Info("cidr allocated", "cidr", free, "owner", owner, "attempt", attempt+1)
			return free, nil
		}
		// Lost the race; another writer bumped the version - re-read and retry.
		p.log.Warn("cidr CAS conflict; retrying", "owner", owner, "attempt", attempt+1)
	}
	return "", ErrCASRetriesExhausted
}

// Release frees every CIDR held by owner. Idempotent (no-op if owner holds none).
func (p *Pool) Release(ctx context.Context, owner string) error {
	if owner == "" {
		return fmt.Errorf("ipam: owner must be non-empty")
	}
	for attempt := 0; attempt < p.maxRetries; attempt++ {
		pool, version, err := p.kv.Read(ctx)
		if err != nil {
			return fmt.Errorf("ipam: read pool: %w", err)
		}
		changed := false
		for cidr, o := range pool {
			if o == owner {
				pool[cidr] = ""
				changed = true
			}
		}
		if !changed {
			return nil
		}
		conflict, err := p.kv.CASWrite(ctx, pool, version)
		if err != nil {
			return fmt.Errorf("ipam: write pool: %w", err)
		}
		if !conflict {
			p.log.Info("cidr released", "owner", owner)
			return nil
		}
	}
	return ErrCASRetriesExhausted
}

// ownedBy returns the CIDR allocated to owner, or "".
func ownedBy(pool map[string]string, owner string) string {
	for cidr, o := range pool {
		if o == owner {
			return cidr
		}
	}
	return ""
}

// firstFree returns the smallest free CIDR (deterministic ordering for stable
// allocation), or "" if none are free.
func firstFree(pool map[string]string) string {
	free := make([]string, 0, len(pool))
	for cidr, owner := range pool {
		if strings.TrimSpace(owner) == "" {
			free = append(free, cidr)
		}
	}
	if len(free) == 0 {
		return ""
	}
	sort.Strings(free)
	return free[0]
}
