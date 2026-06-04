package animation

import "testing"

func TestCompileManifestBuildsDefinitionAndResolver(t *testing.T) {
	def, err := CompileManifest(minimalRawManifest())
	if err != nil {
		t.Fatalf("CompileManifest() error = %v", err)
	}
	if def.ID != "pet-test" {
		t.Fatalf("definition ID = %q, want pet-test", def.ID)
	}
	if def.Name != "pet-test" {
		t.Fatalf("defaulted definition Name = %q, want pet-test", def.Name)
	}
	if def.Scale != 1 {
		t.Fatalf("defaulted definition Scale = %v, want 1", def.Scale)
	}
	if len(def.Clips) != 3 {
		t.Fatalf("clip count = %d, want 3", len(def.Clips))
	}
	clip, ok := def.Resolver.Resolve("left_click", nil)
	if !ok {
		t.Fatal("Resolve(left_click) ok = false")
	}
	if clip.Name != "wave" {
		t.Fatalf("Resolve(left_click) = %q, want wave", clip.Name)
	}
}

func TestCompileManifestRejectsUnknownClipTag(t *testing.T) {
	raw := minimalRawManifest()
	clip := raw.Animations["wave"]
	clip.Tags = append(clip.Tags, "not_registered")
	raw.Animations["wave"] = clip
	if _, err := CompileManifest(raw); err == nil {
		t.Fatal("CompileManifest() error = nil, want unknown tag error")
	}
}

func TestCompileManifestRejectsIntentWithoutCandidates(t *testing.T) {
	raw := minimalRawManifest()
	raw.Intents["missing"] = RawIntentProfile{Fallbacks: []RawIntentQuery{{Required: []string{"negative"}}}}
	if _, err := CompileManifest(raw); err == nil {
		t.Fatal("CompileManifest() error = nil, want no candidates error")
	}
}

func TestParseManifestRejectsWrongSchema(t *testing.T) {
	data := []byte(`{"schema":4,"id":"old"}`)
	if _, err := ParseManifest(data); err == nil {
		t.Fatal("ParseManifest() error = nil, want schema error")
	}
}

func minimalRawManifest() RawManifest {
	return RawManifest{
		Schema:           ManifestSchemaV5,
		ID:               "pet-test",
		FrameWidth:       256,
		FrameHeight:      256,
		AnimationDir:     "animations",
		DefaultAnimation: "idle",
		Tags: []string{
			"idle", "state", "stationary",
			"reaction", "positive", "friendly",
			"locomotion", "slow", "negative",
		},
		Animations: map[string]RawAnimationClip{
			"idle": {
				File:     "idle.png",
				FPS:      4,
				Frames:   8,
				Tags:     []string{"idle", "state", "stationary"},
				Priority: 10,
			},
			"wave": {
				File:       "wave.png",
				FPS:        5,
				Frames:     8,
				DurationMS: 1200,
				Tags:       []string{"reaction", "positive", "friendly", "stationary"},
				Priority:   70,
			},
			"walk": {
				File:         "walk.png",
				FPS:          6,
				Frames:       8,
				Loop:         true,
				Tags:         []string{"locomotion", "slow"},
				Priority:     30,
				SpeedPxS:     40,
				NativeFacing: "right",
			},
		},
		Intents: map[string]RawIntentProfile{
			"idle": {
				Fallbacks: []RawIntentQuery{{Required: []string{"idle"}}},
			},
			"left_click": {
				Fallbacks: []RawIntentQuery{
					{Required: []string{"reaction"}, Preferred: []string{"positive", "friendly"}},
					{Required: []string{"idle"}},
				},
			},
			"locomotion_slow": {
				Fallbacks: []RawIntentQuery{
					{Required: []string{"locomotion"}, Preferred: []string{"slow"}},
					{Required: []string{"idle"}},
				},
			},
		},
	}
}
