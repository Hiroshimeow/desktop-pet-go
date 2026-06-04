package runtimev2

import (
	"testing"
	"time"

	"desktop-pet-lite-go/internal/animation"
	"desktop-pet-lite-go/internal/input"
	"desktop-pet-lite-go/internal/pet"
)

func TestEngineClickFlowsThroughBrainResolverPlayer(t *testing.T) {
	engine := NewEngine(testDefinition(t), EngineConfig{})
	now := time.Unix(100, 0)
	engine.HandleRaw(input.RawEvent{Kind: input.RawPointerDown, Button: input.ButtonLeft, X: 0, Y: 0, At: now})
	result := engine.HandleRaw(input.RawEvent{Kind: input.RawPointerUp, Button: input.ButtonLeft, X: 0, Y: 0, At: now.Add(time.Millisecond)})
	if result.Decision.Intent != pet.IntentLeftClick {
		t.Fatalf("intent = %q, want left_click", result.Decision.Intent)
	}
	if result.ResolvedClip != "wave" {
		t.Fatalf("resolved clip = %q, want wave", result.ResolvedClip)
	}
	if result.Player.ClipName != "wave" {
		t.Fatalf("player clip = %q, want wave", result.Player.ClipName)
	}
}

func TestEngineDragFlowsThroughHoldAndEnd(t *testing.T) {
	now := time.Unix(100, 0)
	engine := NewEngine(testDefinition(t), EngineConfig{Gesture: input.GestureConfig{DragThresholdPx: 5, HoldRepeat: 500 * time.Millisecond}, Brain: pet.BrainConfig{DragHoldRepeat: 500 * time.Millisecond}, Cooldown: time.Second})
	engine.HandleRaw(input.RawEvent{Kind: input.RawPointerDown, Button: input.ButtonLeft, X: 0, Y: 0, At: now})
	start := engine.HandleRaw(input.RawEvent{Kind: input.RawPointerMove, X: 10, Y: 0, At: now.Add(time.Millisecond)})
	if start.Decision.Intent != pet.IntentDragStart || start.ResolvedClip != "angry" {
		t.Fatalf("drag start = intent %q clip %q, want drag_start angry", start.Decision.Intent, start.ResolvedClip)
	}
	hold := engine.Tick(now.Add(600*time.Millisecond), 16*time.Millisecond)
	if hold.Decision.Intent != pet.IntentDragHold || hold.ResolvedClip != "cry" {
		t.Fatalf("drag hold = intent %q clip %q, want drag_hold cry", hold.Decision.Intent, hold.ResolvedClip)
	}
	end := engine.HandleRaw(input.RawEvent{Kind: input.RawPointerUp, Button: input.ButtonLeft, X: 10, Y: 0, At: now.Add(time.Second)})
	if end.Decision.Intent != pet.IntentDragEnd || end.ResolvedClip != "wave" {
		t.Fatalf("drag end = intent %q clip %q, want drag_end wave", end.Decision.Intent, end.ResolvedClip)
	}
}

func TestEngineTickResolvesLocomotionIntent(t *testing.T) {
	engine := NewEngine(testDefinition(t), EngineConfig{Brain: pet.BrainConfig{SlowDistanceThreshold: 100}})
	engine.SetState(State{X: 0, TargetX: 300, HasRoamTarget: true})
	result := engine.Tick(time.Unix(100, 0), 16*time.Millisecond)
	if result.Decision.Intent != pet.IntentLocomotionFast {
		t.Fatalf("intent = %q, want locomotion_fast", result.Decision.Intent)
	}
	if result.ResolvedClip != "run" {
		t.Fatalf("resolved clip = %q, want run", result.ResolvedClip)
	}
}

func testDefinition(t *testing.T) *animation.Definition {
	t.Helper()
	def, err := animation.CompileManifest(animation.RawManifest{
		Schema:           animation.ManifestSchemaV5,
		ID:               "runtime-test",
		FrameWidth:       64,
		FrameHeight:      64,
		AnimationDir:     "animations",
		DefaultAnimation: "idle",
		Tags:             []string{"idle", "state", "stationary", "reaction", "positive", "friendly", "emotion", "negative", "held", "protest", "locomotion", "slow", "fast"},
		Animations: map[string]animation.RawAnimationClip{
			"idle":  {File: "idle.png", FPS: 4, Frames: 4, Tags: []string{"idle", "state", "stationary"}, Priority: 10},
			"wave":  {File: "wave.png", FPS: 5, Frames: 4, Tags: []string{"reaction", "positive", "friendly", "stationary"}, Priority: 70},
			"angry": {File: "angry.png", FPS: 5, Frames: 4, Tags: []string{"emotion", "negative", "held", "protest", "stationary"}, Priority: 80},
			"cry":   {File: "cry.png", FPS: 5, Frames: 4, Tags: []string{"emotion", "negative", "held", "protest", "stationary"}, Priority: 60},
			"walk":  {File: "walk.png", FPS: 6, Frames: 4, Loop: true, Tags: []string{"locomotion", "slow"}, Priority: 30},
			"run":   {File: "run.png", FPS: 8, Frames: 4, Loop: true, Tags: []string{"locomotion", "fast"}, Priority: 40},
		},
		Intents: map[string]animation.RawIntentProfile{
			"idle":            {Fallbacks: []animation.RawIntentQuery{{Required: []string{"idle"}}}},
			"left_click":      {Fallbacks: []animation.RawIntentQuery{{Required: []string{"reaction"}, Preferred: []string{"positive", "friendly"}}, {Required: []string{"idle"}}}},
			"right_click":     {Fallbacks: []animation.RawIntentQuery{{Required: []string{"reaction"}}, {Required: []string{"idle"}}}},
			"drag_start":      {Fallbacks: []animation.RawIntentQuery{{Required: []string{"emotion", "negative", "held"}}, {Required: []string{"idle"}}}},
			"drag_hold":       {Fallbacks: []animation.RawIntentQuery{{Required: []string{"emotion", "negative", "held"}}, {Required: []string{"idle"}}}},
			"drag_end":        {Fallbacks: []animation.RawIntentQuery{{Required: []string{"reaction"}, Preferred: []string{"positive"}}, {Required: []string{"idle"}}}},
			"locomotion_slow": {Fallbacks: []animation.RawIntentQuery{{Required: []string{"locomotion"}, Preferred: []string{"slow"}}, {Required: []string{"idle"}}}},
			"locomotion_fast": {Fallbacks: []animation.RawIntentQuery{{Required: []string{"locomotion"}, Preferred: []string{"fast"}}, {Required: []string{"locomotion"}, Preferred: []string{"slow"}}, {Required: []string{"idle"}}}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return def
}
