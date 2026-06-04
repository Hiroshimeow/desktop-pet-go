package animation

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
)

const ManifestSchemaV5 = 5

type RawManifest struct {
	Schema           int                         `json:"schema"`
	ID               string                      `json:"id"`
	Name             string                      `json:"name"`
	Scale            float64                     `json:"scale"`
	FrameWidth       int                         `json:"frame_width"`
	FrameHeight      int                         `json:"frame_height"`
	AnimationDir     string                      `json:"animation_dir"`
	DefaultAnimation string                      `json:"default_animation"`
	Tags             []string                    `json:"tags"`
	Animations       map[string]RawAnimationClip `json:"animations"`
	Intents          map[string]RawIntentProfile `json:"intents"`
}

type RawAnimationClip struct {
	File         string   `json:"file"`
	FPS          int      `json:"fps"`
	Frames       int      `json:"frames"`
	Loop         bool     `json:"loop"`
	DurationMS   int      `json:"duration_ms"`
	Tags         []string `json:"tags"`
	Priority     int      `json:"priority"`
	SpeedPxS     float64  `json:"speed_px_s"`
	NativeFacing string   `json:"native_facing"`
}

type RawIntentProfile struct {
	Fallbacks []RawIntentQuery `json:"fallbacks"`
}

type RawIntentQuery struct {
	Required       []string `json:"required"`
	Preferred      []string `json:"preferred"`
	Excluded       []string `json:"excluded"`
	PreferredBonus int      `json:"preferred_bonus"`
	BaseScore      int      `json:"base_score"`
}

type Definition struct {
	ID               string
	Name             string
	Scale            float64
	FrameWidth       int
	FrameHeight      int
	AnimationDir     string
	DefaultAnimation string
	Tags             *TagRegistry
	Clips            map[string]ClipDefinition
	Resolver         *Resolver
	Intents          map[string]CompiledIntent
}

type ClipDefinition struct {
	Clip
	File         string
	FPS          int
	Frames       int
	Loop         bool
	DurationMS   int
	SpeedPxS     float64
	NativeFacing string
}

func LoadManifestFile(path string) (*Definition, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ParseManifest(data)
}

func ParseManifest(data []byte) (*Definition, error) {
	var raw RawManifest
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	return CompileManifest(raw)
}

func CompileManifest(raw RawManifest) (*Definition, error) {
	if raw.Schema != ManifestSchemaV5 {
		return nil, fmt.Errorf("animation manifest schema = %d, want %d", raw.Schema, ManifestSchemaV5)
	}
	if raw.ID == "" {
		return nil, fmt.Errorf("manifest id is required")
	}
	if raw.FrameWidth <= 0 || raw.FrameHeight <= 0 {
		return nil, fmt.Errorf("frame_width and frame_height must be positive")
	}
	if raw.AnimationDir == "" {
		return nil, fmt.Errorf("animation_dir is required")
	}
	if raw.DefaultAnimation == "" {
		return nil, fmt.Errorf("default_animation is required")
	}
	if len(raw.Tags) == 0 {
		return nil, fmt.Errorf("at least one semantic tag is required")
	}
	if len(raw.Animations) == 0 {
		return nil, fmt.Errorf("at least one animation is required")
	}
	registry, err := NewTagRegistry(raw.Tags...)
	if err != nil {
		return nil, err
	}
	clips := make(map[string]ClipDefinition, len(raw.Animations))
	resolverClips := make([]Clip, 0, len(raw.Animations))
	for _, name := range sortedRawAnimationNames(raw.Animations) {
		rawClip := raw.Animations[name]
		clip, err := compileRawClip(registry, name, rawClip)
		if err != nil {
			return nil, err
		}
		clips[name] = clip
		resolverClips = append(resolverClips, clip.Clip)
	}
	if _, ok := clips[raw.DefaultAnimation]; !ok {
		return nil, fmt.Errorf("default_animation %q not found", raw.DefaultAnimation)
	}
	if len(raw.Intents) == 0 {
		return nil, fmt.Errorf("at least one intent is required")
	}
	compiledIntents := make(map[string]CompiledIntent, len(raw.Intents))
	resolverInputs := make([]CompiledIntent, 0, len(raw.Intents))
	for _, intentName := range sortedRawIntentNames(raw.Intents) {
		intent, err := compileRawIntent(registry, intentName, raw.Intents[intentName], resolverClips)
		if err != nil {
			return nil, err
		}
		compiledIntents[intentName] = intent
		resolverInputs = append(resolverInputs, intent)
	}
	resolver, err := NewResolver(resolverInputs...)
	if err != nil {
		return nil, err
	}
	name := raw.Name
	if name == "" {
		name = raw.ID
	}
	scale := raw.Scale
	if scale <= 0 {
		scale = 1
	}
	return &Definition{
		ID:               raw.ID,
		Name:             name,
		Scale:            scale,
		FrameWidth:       raw.FrameWidth,
		FrameHeight:      raw.FrameHeight,
		AnimationDir:     raw.AnimationDir,
		DefaultAnimation: raw.DefaultAnimation,
		Tags:             registry,
		Clips:            clips,
		Resolver:         resolver,
		Intents:          compiledIntents,
	}, nil
}

func compileRawClip(registry *TagRegistry, name string, raw RawAnimationClip) (ClipDefinition, error) {
	if name == "" {
		return ClipDefinition{}, fmt.Errorf("animation name must not be empty")
	}
	if raw.File == "" {
		return ClipDefinition{}, fmt.Errorf("animation %q missing file", name)
	}
	if raw.FPS <= 0 {
		return ClipDefinition{}, fmt.Errorf("animation %q fps must be positive", name)
	}
	if raw.Frames <= 0 {
		return ClipDefinition{}, fmt.Errorf("animation %q frames must be positive", name)
	}
	if len(raw.Tags) == 0 {
		return ClipDefinition{}, fmt.Errorf("animation %q must declare semantic tags", name)
	}
	mask, err := registry.Mask(raw.Tags...)
	if err != nil {
		return ClipDefinition{}, fmt.Errorf("animation %q: %w", name, err)
	}
	return ClipDefinition{
		Clip: Clip{
			Name:     name,
			Tags:     mask,
			Priority: raw.Priority,
		},
		File:         raw.File,
		FPS:          raw.FPS,
		Frames:       raw.Frames,
		Loop:         raw.Loop,
		DurationMS:   raw.DurationMS,
		SpeedPxS:     raw.SpeedPxS,
		NativeFacing: raw.NativeFacing,
	}, nil
}

func compileRawIntent(registry *TagRegistry, name string, raw RawIntentProfile, clips []Clip) (CompiledIntent, error) {
	if name == "" {
		return CompiledIntent{}, fmt.Errorf("intent name must not be empty")
	}
	if len(raw.Fallbacks) == 0 {
		return CompiledIntent{}, fmt.Errorf("intent %q must declare fallback queries", name)
	}
	fallbacks := make([]IntentQuery, 0, len(raw.Fallbacks))
	for i, fallback := range raw.Fallbacks {
		query, err := compileRawQuery(registry, fallback)
		if err != nil {
			return CompiledIntent{}, fmt.Errorf("intent %q fallback %d: %w", name, i, err)
		}
		fallbacks = append(fallbacks, query)
	}
	compiled, err := CompileIntent(IntentDefinition{Name: name, Fallbacks: fallbacks}, clips)
	if err != nil {
		return CompiledIntent{}, err
	}
	if len(compiled.Groups) == 0 {
		return CompiledIntent{}, fmt.Errorf("intent %q has no matching animation candidates", name)
	}
	return compiled, nil
}

func compileRawQuery(registry *TagRegistry, raw RawIntentQuery) (IntentQuery, error) {
	required, err := registry.Mask(raw.Required...)
	if err != nil {
		return IntentQuery{}, err
	}
	preferred, err := registry.Mask(raw.Preferred...)
	if err != nil {
		return IntentQuery{}, err
	}
	excluded, err := registry.Mask(raw.Excluded...)
	if err != nil {
		return IntentQuery{}, err
	}
	return IntentQuery{
		Required:       required,
		Preferred:      preferred,
		Excluded:       excluded,
		PreferredBonus: raw.PreferredBonus,
		BaseScore:      raw.BaseScore,
	}, nil
}

func sortedRawAnimationNames(anims map[string]RawAnimationClip) []string {
	names := make([]string, 0, len(anims))
	for name := range anims {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func sortedRawIntentNames(intents map[string]RawIntentProfile) []string {
	names := make([]string, 0, len(intents))
	for name := range intents {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
