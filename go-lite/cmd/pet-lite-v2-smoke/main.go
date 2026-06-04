package main

import (
	"fmt"
	"time"

	"desktop-pet-lite-go/internal/animation"
	"desktop-pet-lite-go/internal/input"
	"desktop-pet-lite-go/internal/pet"
	"desktop-pet-lite-go/internal/runtimev2"
)

func main() {
	definition, err := smokeDefinition()
	if err != nil {
		panic(err)
	}
	engine := runtimev2.NewEngine(definition, runtimev2.EngineConfig{
		Gesture: input.GestureConfig{DragThresholdPx: 5, HoldRepeat: 500 * time.Millisecond},
		Brain:   pet.BrainConfig{DragHoldRepeat: 500 * time.Millisecond, SlowDistanceThreshold: 100},
	})
	now := time.Unix(100, 0)
	engine.HandleRaw(input.RawEvent{Kind: input.RawPointerDown, Button: input.ButtonLeft, X: 0, Y: 0, At: now})
	printResult("click", engine.HandleRaw(input.RawEvent{Kind: input.RawPointerUp, Button: input.ButtonLeft, X: 0, Y: 0, At: now.Add(time.Millisecond)}))
	engine.HandleRaw(input.RawEvent{Kind: input.RawPointerDown, Button: input.ButtonLeft, X: 0, Y: 0, At: now.Add(time.Second)})
	printResult("drag_start", engine.HandleRaw(input.RawEvent{Kind: input.RawPointerMove, X: 10, Y: 0, At: now.Add(time.Second + time.Millisecond)}))
	printResult("drag_hold", engine.Tick(now.Add(2*time.Second), 16*time.Millisecond))
	printResult("drag_end", engine.HandleRaw(input.RawEvent{Kind: input.RawPointerUp, Button: input.ButtonLeft, X: 10, Y: 0, At: now.Add(3 * time.Second)}))
	engine.SetState(runtimev2.State{X: 0, TargetX: 300, HasRoamTarget: true})
	printResult("locomotion", engine.Tick(now.Add(5*time.Second), 16*time.Millisecond))
}

func printResult(label string, result runtimev2.StepResult) {
	fmt.Printf("%s intent=%s clip=%s player=%s frame=%d resolved=%v\n", label, result.Decision.Intent, result.ResolvedClip, result.Player.ClipName, result.Player.FrameIndex, result.Resolved)
}

func smokeDefinition() (*animation.Definition, error) {
	return animation.CompileManifest(animation.RawManifest{
		Schema:           animation.ManifestSchemaV5,
		ID:               "v2-smoke",
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
}
