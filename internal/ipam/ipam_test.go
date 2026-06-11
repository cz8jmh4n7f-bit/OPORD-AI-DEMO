package ipam

import (
	"context"
	"testing"
)

// fakeKV is an in-memory KV with a version counter. injectConflicts forces the
// next N CASWrites to report a conflict (simulating concurrent writers).
type fakeKV struct {
	pool            map[string]string
	version         int
	injectConflicts int
	writes          int
}

func (f *fakeKV) Read(_ context.Context) (map[string]string, int, error) {
	cp := make(map[string]string, len(f.pool))
	for k, v := range f.pool {
		cp[k] = v
	}
	return cp, f.version, nil
}

func (f *fakeKV) CASWrite(_ context.Context, pool map[string]string, expected int) (bool, error) {
	f.writes++
	if f.injectConflicts > 0 {
		f.injectConflicts--
		f.version++ // someone else bumped it
		return true, nil
	}
	if expected != f.version {
		return true, nil
	}
	f.pool = pool
	f.version++
	return false, nil
}

func newFake() *fakeKV {
	return &fakeKV{pool: map[string]string{
		"10.16.0.0/22": "",
		"10.16.4.0/22": "",
		"10.16.8.0/22": "",
	}, version: 1}
}

func TestAllocate_picksFreeDeterministic(t *testing.T) {
	p := New(newFake(), nil)
	got, err := p.Allocate(context.Background(), "acct-1")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got != "10.16.0.0/22" {
		t.Fatalf("want smallest free 10.16.0.0/22, got %s", got)
	}
}

func TestAllocate_idempotentSameOwner(t *testing.T) {
	kv := newFake()
	p := New(kv, nil)
	first, _ := p.Allocate(context.Background(), "acct-1")
	second, err := p.Allocate(context.Background(), "acct-1")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if first != second {
		t.Fatalf("idempotency broken: %s != %s", first, second)
	}
	// Only the first allocation should have mutated the pool.
	allocated := 0
	for _, o := range kv.pool {
		if o == "acct-1" {
			allocated++
		}
	}
	if allocated != 1 {
		t.Fatalf("want exactly 1 block for acct-1, got %d", allocated)
	}
}

func TestAllocate_distinctOwners(t *testing.T) {
	p := New(newFake(), nil)
	a, _ := p.Allocate(context.Background(), "acct-1")
	b, _ := p.Allocate(context.Background(), "acct-2")
	if a == b {
		t.Fatalf("distinct owners got the same CIDR %s", a)
	}
}

func TestAllocate_exhaustion(t *testing.T) {
	p := New(newFake(), nil)
	for i, owner := range []string{"a", "b", "c"} {
		if _, err := p.Allocate(context.Background(), owner); err != nil {
			t.Fatalf("alloc %d failed: %v", i, err)
		}
	}
	if _, err := p.Allocate(context.Background(), "d"); err != ErrPoolExhausted {
		t.Fatalf("want ErrPoolExhausted, got %v", err)
	}
}

func TestAllocate_retriesPastCASConflict(t *testing.T) {
	kv := newFake()
	kv.injectConflicts = 2 // first two writes collide, third wins
	p := New(kv, nil)
	got, err := p.Allocate(context.Background(), "acct-1")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got == "" {
		t.Fatal("expected a CIDR after retrying past conflicts")
	}
	if kv.writes != 3 {
		t.Fatalf("want 3 write attempts (2 conflicts + 1 success), got %d", kv.writes)
	}
}

func TestRelease_freesAndIdempotent(t *testing.T) {
	kv := newFake()
	p := New(kv, nil)
	cidr, _ := p.Allocate(context.Background(), "acct-1")
	if err := p.Release(context.Background(), "acct-1"); err != nil {
		t.Fatalf("release: %v", err)
	}
	if kv.pool[cidr] != "" {
		t.Fatalf("cidr %s not freed", cidr)
	}
	// Releasing again is a no-op.
	if err := p.Release(context.Background(), "acct-1"); err != nil {
		t.Fatalf("second release should be no-op: %v", err)
	}
}
