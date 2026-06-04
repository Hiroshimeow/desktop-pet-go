package animation

import (
	"testing"
	"time"
)

type fakeRNG struct{ values []int }

func (f *fakeRNG) Intn(n int) int {
	if len(f.values) == 0 {
		return 0
	}
	v := f.values[0]
	f.values = f.values[1:]
	if v < 0 {
		return 0
	}
	return v % n
}

func TestResolverWeightedRandomIsDeterministicWithInjectedRNG(t *testing.T) {
	intent := CompiledIntent{
		Name: "idle",
		Groups: [][]Candidate{{
			{Clip: Clip{Name: "low"}, Score: 1},
			{Clip: Clip{Name: "high"}, Score: 9},
		}},
	}
	resolver, err := NewResolver(intent)
	if err != nil {
		t.Fatal(err)
	}
	clip, ok := resolver.Resolve("idle", &fakeRNG{values: []int{1}})
	if !ok {
		t.Fatal("Resolve() ok = false")
	}
	if clip.Name != "high" {
		t.Fatalf("Resolve() clip = %q, want high", clip.Name)
	}
}

func TestResolverUnknownIntentReturnsFalse(t *testing.T) {
	resolver, err := NewResolver(CompiledIntent{Name: "idle"})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := resolver.Resolve("missing", nil); ok {
		t.Fatal("Resolve() ok = true for missing intent")
	}
}

func TestResolverCooldownAvoidsRepeatWhenAlternativeExists(t *testing.T) {
	now := time.Unix(100, 0)
	resolver, err := NewResolver(CompiledIntent{
		Name: "reaction",
		Groups: [][]Candidate{{
			{Clip: Clip{Name: "wave"}, Score: 100},
			{Clip: Clip{Name: "cheer"}, Score: 1},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	resolver.RecordSelection("wave", now, time.Second, 2)
	clip, ok := resolver.ResolveWithContext("reaction", ResolveContext{Now: now.Add(100 * time.Millisecond)}, &fakeRNG{values: []int{0}})
	if !ok {
		t.Fatal("ResolveWithContext() ok = false")
	}
	if clip.Name != "cheer" {
		t.Fatalf("ResolveWithContext() clip = %q, want cheer", clip.Name)
	}
}

func TestResolverCooldownFallsThroughToNextFallbackGroup(t *testing.T) {
	now := time.Unix(100, 0)
	resolver, err := NewResolver(CompiledIntent{
		Name: "left_click",
		Groups: [][]Candidate{
			{{Clip: Clip{Name: "wave"}, Score: 100}},
			{{Clip: Clip{Name: "idle"}, Score: 1}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	resolver.RecordSelection("wave", now, time.Second, 1)
	clip, ok := resolver.ResolveWithContext("left_click", ResolveContext{Now: now.Add(100 * time.Millisecond)}, nil)
	if !ok {
		t.Fatal("ResolveWithContext() ok = false")
	}
	if clip.Name != "idle" {
		t.Fatalf("ResolveWithContext() clip = %q, want idle", clip.Name)
	}
}

func TestResolverAllowCooldownOnlyUsesPenalizedCoolingCandidate(t *testing.T) {
	now := time.Unix(100, 0)
	resolver, err := NewResolver(CompiledIntent{
		Name: "idle",
		Groups: [][]Candidate{{
			{Clip: Clip{Name: "idle"}, Score: 50},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	resolver.RecordSelection("idle", now, time.Second, 1)
	clip, ok := resolver.ResolveWithContext("idle", ResolveContext{
		Now:               now.Add(100 * time.Millisecond),
		AllowCooldownOnly: true,
		CooldownPenalty:   100,
	}, nil)
	if !ok {
		t.Fatal("ResolveWithContext() ok = false")
	}
	if clip.Name != "idle" {
		t.Fatalf("ResolveWithContext() clip = %q, want idle", clip.Name)
	}
}

func TestResolverRecentPenaltyChangesWeightedRange(t *testing.T) {
	now := time.Unix(100, 0)
	resolver, err := NewResolver(CompiledIntent{
		Name: "reaction",
		Groups: [][]Candidate{{
			{Clip: Clip{Name: "wave"}, Score: 10},
			{Clip: Clip{Name: "cheer"}, Score: 10},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	resolver.RecordSelection("wave", now, 0, 2)
	clip, ok := resolver.ResolveWithContext("reaction", ResolveContext{
		Now:           now.Add(time.Millisecond),
		RecentPenalty: 10,
	}, &fakeRNG{values: []int{1}})
	if !ok {
		t.Fatal("ResolveWithContext() ok = false")
	}
	if clip.Name != "cheer" {
		t.Fatalf("ResolveWithContext() clip = %q, want cheer", clip.Name)
	}
}

func TestResolverRecordSelectionUpdatesCooldownAndRecentHistory(t *testing.T) {
	now := time.Unix(100, 0)
	resolver, err := NewResolver(CompiledIntent{
		Name: "reaction",
		Groups: [][]Candidate{{
			{Clip: Clip{Name: "wave"}, Score: 100},
			{Clip: Clip{Name: "cheer"}, Score: 1},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	clip, ok := resolver.ResolveWithContext("reaction", ResolveContext{
		Now:             now,
		Cooldown:        time.Second,
		RecentLimit:     1,
		RecordSelection: true,
	}, nil)
	if !ok {
		t.Fatal("ResolveWithContext() ok = false")
	}
	if clip.Name != "wave" {
		t.Fatalf("first clip = %q, want wave", clip.Name)
	}
	clip, ok = resolver.ResolveWithContext("reaction", ResolveContext{Now: now.Add(time.Millisecond)}, nil)
	if !ok {
		t.Fatal("second ResolveWithContext() ok = false")
	}
	if clip.Name != "cheer" {
		t.Fatalf("second clip = %q, want cheer because wave is cooling", clip.Name)
	}
}
