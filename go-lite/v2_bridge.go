package main

import (
	"fmt"
	"strings"

	"desktop-pet-lite-go/internal/animation"
)

func compileLegacyManifestV2(manifest PetManifest) (*animation.Definition, error) {
	tags := []string{"idle", "state", "stationary", "reaction", "positive", "friendly", "recover", "emotion", "negative", "held", "protest", "curious", "locomotion", "slow", "fast"}
	raw := animation.RawManifest{
		Schema:           animation.ManifestSchemaV5,
		ID:               manifest.ID,
		Name:             manifest.Name,
		Scale:            manifest.Scale,
		FrameWidth:       manifest.FrameWidth,
		FrameHeight:      manifest.FrameHeight,
		AnimationDir:     manifest.AnimationDir,
		DefaultAnimation: manifest.DefaultAnimation,
		Tags:             tags,
		Animations:       map[string]animation.RawAnimationClip{},
		Intents:          defaultRuntimeV2Intents(),
	}
	for name, def := range manifest.Animations {
		tagSet := inferLegacySemanticTags(name, def, manifest.DefaultAnimation)
		raw.Animations[name] = animation.RawAnimationClip{
			File:         def.File,
			FPS:          def.FPS,
			Frames:       frameCountOf(manifest, def),
			Loop:         def.Loop || def.Locomotion,
			DurationMS:   durationOf(def),
			Tags:         tagSet,
			Priority:     inferLegacyPriority(name, def, manifest.DefaultAnimation),
			SpeedPxS:     def.SpeedPxS,
			NativeFacing: def.NativeFacing,
		}
	}
	definition, err := animation.CompileManifest(raw)
	if err != nil {
		return nil, fmt.Errorf("compile legacy manifest as v2 definition pet=%s: %w", manifest.ID, err)
	}
	return definition, nil
}

func inferLegacySemanticTags(name string, def AnimationDef, defaultAnimation string) []string {
	set := map[string]bool{"stationary": true}
	lower := strings.ToLower(name)
	kind := strings.ToLower(def.Kind)
	if name == defaultAnimation || lower == "idle" {
		set["idle"] = true
		set["state"] = true
	}
	if def.Locomotion || kind == "move" || strings.Contains(lower, "walk") || strings.Contains(lower, "run") {
		delete(set, "stationary")
		set["locomotion"] = true
		if strings.Contains(lower, "run") || def.SpeedPxS >= 55 {
			set["fast"] = true
		} else {
			set["slow"] = true
		}
	}
	if kind == "state" || strings.Contains(lower, "sleep") || strings.Contains(lower, "sit") || strings.Contains(lower, "think") {
		set["state"] = true
	}
	if kind == "reaction" || strings.Contains(lower, "wave") || strings.Contains(lower, "cheer") || strings.Contains(lower, "dance") {
		set["reaction"] = true
	}
	if kind == "emotion" || strings.Contains(lower, "happy") || strings.Contains(lower, "cry") || strings.Contains(lower, "angry") || strings.Contains(lower, "scared") || strings.Contains(lower, "dizzy") || strings.Contains(lower, "shy") || strings.Contains(lower, "surprised") {
		set["emotion"] = true
	}
	if strings.Contains(lower, "happy") || strings.Contains(lower, "wave") || strings.Contains(lower, "cheer") || strings.Contains(lower, "dance") {
		set["positive"] = true
		set["friendly"] = true
	}
	if strings.Contains(lower, "cry") || strings.Contains(lower, "angry") || strings.Contains(lower, "scared") || strings.Contains(lower, "dizzy") {
		set["negative"] = true
		set["held"] = true
		set["protest"] = true
	}
	if strings.Contains(lower, "think") || strings.Contains(lower, "surprised") || strings.Contains(lower, "shy") {
		set["curious"] = true
	}
	if strings.Contains(lower, "wave") {
		set["recover"] = true
	}
	if !set["locomotion"] && !set["reaction"] && !set["emotion"] && !set["state"] {
		set["state"] = true
	}
	return orderedKnownTags(set)
}

func inferLegacyPriority(name string, def AnimationDef, defaultAnimation string) int {
	lower := strings.ToLower(name)
	if name == defaultAnimation || lower == "idle" {
		return 10
	}
	if def.Locomotion || strings.Contains(lower, "walk") || strings.Contains(lower, "run") {
		if strings.Contains(lower, "run") || def.SpeedPxS >= 55 {
			return 45
		}
		return 35
	}
	if strings.Contains(lower, "angry") || strings.Contains(lower, "cry") || strings.Contains(lower, "scared") || strings.Contains(lower, "dizzy") {
		return 85
	}
	if strings.Contains(lower, "wave") || strings.Contains(lower, "happy") || strings.Contains(lower, "cheer") {
		return 70
	}
	return 30
}

func orderedKnownTags(set map[string]bool) []string {
	known := []string{"idle", "state", "stationary", "reaction", "positive", "friendly", "recover", "emotion", "negative", "held", "protest", "curious", "locomotion", "slow", "fast"}
	out := make([]string, 0, len(set))
	for _, tag := range known {
		if set[tag] {
			out = append(out, tag)
		}
	}
	return out
}

func defaultRuntimeV2Intents() map[string]animation.RawIntentProfile {
	return map[string]animation.RawIntentProfile{
		"idle":            {Fallbacks: []animation.RawIntentQuery{{Required: []string{"idle"}}, {Required: []string{"state", "stationary"}}, {Required: []string{"stationary"}}}},
		"left_click":      {Fallbacks: []animation.RawIntentQuery{{Required: []string{"reaction"}, Preferred: []string{"positive", "friendly"}}, {Required: []string{"emotion"}, Preferred: []string{"positive"}}, {Required: []string{"reaction"}}, {Required: []string{"idle"}}}},
		"right_click":     {Fallbacks: []animation.RawIntentQuery{{Required: []string{"reaction"}, Preferred: []string{"curious"}}, {Required: []string{"emotion"}}, {Required: []string{"idle"}}}},
		"drag_start":      {Fallbacks: []animation.RawIntentQuery{{Required: []string{"emotion", "stationary"}, Preferred: []string{"negative", "held", "protest"}}, {Required: []string{"reaction"}, Preferred: []string{"negative"}}, {Required: []string{"idle"}}}},
		"drag_hold":       {Fallbacks: []animation.RawIntentQuery{{Required: []string{"emotion", "stationary"}, Preferred: []string{"negative", "held", "protest"}}, {Required: []string{"emotion"}, Preferred: []string{"negative"}}, {Required: []string{"idle"}}}},
		"drag_end":        {Fallbacks: []animation.RawIntentQuery{{Required: []string{"reaction"}, Preferred: []string{"positive", "friendly", "recover"}}, {Required: []string{"reaction"}}, {Required: []string{"idle"}}}},
		"locomotion_slow": {Fallbacks: []animation.RawIntentQuery{{Required: []string{"locomotion"}, Preferred: []string{"slow"}}, {Required: []string{"locomotion"}}, {Required: []string{"idle"}}}},
		"locomotion_fast": {Fallbacks: []animation.RawIntentQuery{{Required: []string{"locomotion"}, Preferred: []string{"fast"}}, {Required: []string{"locomotion"}, Preferred: []string{"slow"}}, {Required: []string{"locomotion"}}, {Required: []string{"idle"}}}},
		"voice_listening": {Fallbacks: []animation.RawIntentQuery{{Required: []string{"state", "stationary"}, Preferred: []string{"curious"}}, {Required: []string{"idle"}}}},
		"voice_thinking":  {Fallbacks: []animation.RawIntentQuery{{Required: []string{"state", "stationary"}, Preferred: []string{"curious"}}, {Required: []string{"idle"}}}},
		"voice_speaking":  {Fallbacks: []animation.RawIntentQuery{{Required: []string{"reaction"}, Preferred: []string{"friendly", "positive"}}, {Required: []string{"idle"}}}},
		"voice_unknown":   {Fallbacks: []animation.RawIntentQuery{{Required: []string{"emotion"}, Preferred: []string{"curious"}}, {Required: []string{"state", "stationary"}}, {Required: []string{"idle"}}}},
		"voice_error":     {Fallbacks: []animation.RawIntentQuery{{Required: []string{"emotion"}, Preferred: []string{"negative"}}, {Required: []string{"idle"}}}},
	}
}
