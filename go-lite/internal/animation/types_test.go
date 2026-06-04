package animation

import "testing"

func TestTagRegistryMaskAndQueries(t *testing.T) {
	registry, err := NewTagRegistry("idle", "state", "stationary", "reaction", "positive", "friendly", "negative")
	if err != nil {
		t.Fatalf("NewTagRegistry() error = %v", err)
	}

	idle := registry.MustMask("idle", "state", "stationary")
	positiveReaction := registry.MustMask("reaction", "positive", "friendly")
	negative := registry.MustMask("negative")

	query := IntentQuery{Required: registry.MustMask("reaction"), Preferred: registry.MustMask("positive", "friendly"), Excluded: negative}
	if query.Matches(Clip{Name: "idle", Tags: idle}) {
		t.Fatal("idle clip must not match reaction query")
	}
	if !query.Matches(Clip{Name: "wave", Tags: positiveReaction}) {
		t.Fatal("positive friendly reaction clip must match")
	}
	if query.Score(Clip{Name: "wave", Tags: positiveReaction, Priority: 5}) != 25 {
		t.Fatalf("unexpected score: got %d", query.Score(Clip{Name: "wave", Tags: positiveReaction, Priority: 5}))
	}
}

func TestCompileIntentUsesFirstNonEmptyFallbackGroup(t *testing.T) {
	registry, err := NewTagRegistry("idle", "stationary", "reaction", "positive", "friendly")
	if err != nil {
		t.Fatal(err)
	}
	clips := []Clip{
		{Name: "idle", Tags: registry.MustMask("idle", "stationary"), Priority: 1},
		{Name: "wave", Tags: registry.MustMask("reaction", "positive", "friendly", "stationary"), Priority: 3},
	}

	compiled, err := CompileIntent(IntentDefinition{
		Name: "left_click",
		Fallbacks: []IntentQuery{
			{Required: registry.MustMask("reaction"), Preferred: registry.MustMask("positive", "friendly")},
			{Required: registry.MustMask("idle")},
		},
	}, clips)
	if err != nil {
		t.Fatalf("CompileIntent() error = %v", err)
	}
	if len(compiled.Groups) != 2 {
		t.Fatalf("compiled group count = %d, want 2", len(compiled.Groups))
	}
	resolver, err := NewResolver(compiled)
	if err != nil {
		t.Fatalf("NewResolver() error = %v", err)
	}
	clip, ok := resolver.Resolve("left_click", nil)
	if !ok {
		t.Fatal("Resolve() ok = false")
	}
	if clip.Name != "wave" {
		t.Fatalf("Resolve() clip = %q, want wave", clip.Name)
	}
}

func TestCompileIntentSkipsEmptyFallbackGroups(t *testing.T) {
	registry, err := NewTagRegistry("idle", "stationary", "reaction", "positive")
	if err != nil {
		t.Fatal(err)
	}
	clips := []Clip{{Name: "idle", Tags: registry.MustMask("idle", "stationary")}}
	compiled, err := CompileIntent(IntentDefinition{
		Name: "drag_end",
		Fallbacks: []IntentQuery{
			{Required: registry.MustMask("reaction", "positive")},
			{Required: registry.MustMask("idle")},
		},
	}, clips)
	if err != nil {
		t.Fatal(err)
	}
	if len(compiled.Groups) != 1 {
		t.Fatalf("compiled group count = %d, want 1", len(compiled.Groups))
	}
	if compiled.Groups[0][0].Clip.Name != "idle" {
		t.Fatalf("fallback clip = %q, want idle", compiled.Groups[0][0].Clip.Name)
	}
}
