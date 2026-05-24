package main

import "testing"

func TestAutoDiscoverAndSyncedPetManifest(t *testing.T) {
	profile, _, err := loadRuntimeProfile("", "..\\assets", "pet1")
	if err != nil {
		t.Fatal(err)
	}
	if len(profile.ActivePets) < 1 {
		t.Fatalf("expected at least 1 discovered pet group, got %d", len(profile.ActivePets))
	}
	manifest, err := LoadPetManifestSynced("..\\assets\\pet.json", "..\\assets\\pets\\pet1")
	if err != nil {
		t.Fatal(err)
	}
	if manifest.ID != "pet1" {
		t.Fatalf("bad manifest id %q", manifest.ID)
	}
	if manifest.AnimationDir != "animations" {
		t.Fatalf("pet animation_dir must be local to pet folder")
	}
	if !manifest.Animations["walk"].Locomotion || !manifest.Animations["run"].Locomotion {
		t.Fatalf("walk/run must be locomotion")
	}
	if manifest.Animations["cry"].Locomotion || manifest.Animations["angry"].Locomotion {
		t.Fatalf("emotion must not locomote")
	}
}

func TestLoadSpriteStorePerPet(t *testing.T) {
	manifest, err := LoadPetManifestSynced("..\\assets\\pet.json", "..\\assets\\pets\\pet1")
	if err != nil {
		t.Fatal(err)
	}
	store, err := LoadSpriteStore(manifest)
	if err != nil {
		t.Fatal(err)
	}
	strip, x, y, w, h, err := store.FrameRect("run", 4)
	if err != nil {
		t.Fatal(err)
	}
	if strip.Name != "run" || x != 1024 || y != 0 || w != 256 || h != 256 {
		t.Fatalf("bad rect: strip=%s rect=%d,%d,%d,%d", strip.Name, x, y, w, h)
	}
}

func TestPetMovementAndDragEmotionAreSeparated(t *testing.T) {
	manifest, err := LoadPetManifestSynced("..\\assets\\pet.json", "..\\assets\\pets\\pet1")
	if err != nil {
		t.Fatal(err)
	}
	store, err := LoadSpriteStore(manifest)
	if err != nil {
		t.Fatal(err)
	}
	p := NewPet("test", "Test", manifest, store, 1000, 800, 128, 128)
	p.Animation = "walk"
	p.X = 100
	p.TargetX = 400
	y := p.Y
	for i := 0; i < 20; i++ {
		p.Update(0.016, 1000, 128)
	}
	if p.Y != y {
		t.Fatalf("pet moved vertically: %f -> %f", y, p.Y)
	}
	if p.X <= 100 {
		t.Fatalf("pet did not move right: %f", p.X)
	}
	if !manifest.Animations[p.Animation].Locomotion {
		t.Fatalf("movement used non-locomotion animation: %s", p.Animation)
	}
	p.StartDrag()
	oldX := p.X
	p.UpdateDragEmotion()
	p.advanceFrame(0.2)
	if p.X != oldX {
		t.Fatalf("drag emotion should not move by itself")
	}
	if manifest.Animations[p.Animation].Locomotion {
		t.Fatalf("drag should use emotion/state, got %s", p.Animation)
	}
}
