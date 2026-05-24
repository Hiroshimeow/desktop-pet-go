package main

import "fmt"

type HostAction string

const (
	ActionLeftClick  HostAction = "left_click"
	ActionRightClick HostAction = "right_click"
	ActionDragStart  HostAction = "drag_start"
	ActionDragHold   HostAction = "drag_hold"
	ActionDragEnd    HostAction = "drag_end"
	ActionMusicStart HostAction = "music_start"
	ActionMusicStop  HostAction = "music_stop"
)

func AnimationForHostAction(manifest PetManifest, action HostAction) (string, error) {
	mapped, ok := manifest.Interactions[string(action)]
	if !ok {
		return "", fmt.Errorf("unknown host action %q for pet %s", action, manifest.ID)
	}
	if mapped.Animation != "" {
		return mapped.Animation, nil
	}
	if len(mapped.Random) > 0 {
		return mapped.Random[0], nil
	}
	return manifest.DefaultAnimation, nil
}
