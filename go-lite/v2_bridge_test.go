package main

import (
	"testing"
	"time"
)

func TestCompileLegacyManifestV2AndPetActions(t *testing.T) {
	manifest := testManifest(t)
	store, err := LoadSpriteStore(manifest)
	if err != nil {
		t.Fatal(err)
	}
	pet := NewPet("v2-test", "V2 Test", store.Manifest, store, 1000, 800, 128, 128)
	if pet.V2 == nil {
		t.Fatal("expected v2 engine to be initialized")
	}
	pet.TriggerAction("left_click")
	if pet.Animation == "" || pet.Animation == store.Manifest.DefaultAnimation {
		t.Fatalf("left_click should resolve a non-default semantic reaction, got %q", pet.Animation)
	}
	pet.StartDrag()
	startAnimation := pet.Animation
	if startAnimation == "" || store.Manifest.Animations[startAnimation].Locomotion {
		t.Fatalf("drag_start should resolve stationary non-locomotion animation, got %q", startAnimation)
	}
	pet.Update(700.0/1000.0, 1000, 128)
	pet.UpdateDragEmotion()
	if pet.Animation == "" || store.Manifest.Animations[pet.Animation].Locomotion {
		t.Fatalf("drag_hold should keep non-locomotion animation, got %q", pet.Animation)
	}
	pet.EndDrag()
	if pet.Animation == "" || store.Manifest.Animations[pet.Animation].Locomotion {
		t.Fatalf("drag_end should resolve non-locomotion animation, got %q", pet.Animation)
	}
}

func TestPetUpdateUsesV2Locomotion(t *testing.T) {
	manifest := testManifest(t)
	store, err := LoadSpriteStore(manifest)
	if err != nil {
		t.Fatal(err)
	}
	pet := NewPet("v2-move", "V2 Move", store.Manifest, store, 1000, 800, 128, 128)
	if pet.V2 == nil {
		t.Fatal("expected v2 engine to be initialized")
	}
	pet.X = 100
	pet.TargetX = 800
	pet.HasRoamTarget = true
	pet.NextThink = time.Now().Add(time.Hour)
	pet.Update(0.016, 1000, 128)
	if !store.Manifest.Animations[pet.Animation].Locomotion {
		t.Fatalf("v2 update should resolve locomotion animation, got %q", pet.Animation)
	}
	if pet.X <= 100 {
		t.Fatalf("v2 locomotion should move pet right, got x=%f", pet.X)
	}
}
