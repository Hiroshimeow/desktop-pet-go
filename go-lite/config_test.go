package main

import (
	"path/filepath"
	"strings"
	"testing"
)

const testAssetsRoot = "..\\assets"
const testPetID = "pet5"

func testManifest(t *testing.T) PetManifest {
	t.Helper()
	manifest, err := LoadPetManifestMerged(filepath.Join(testAssetsRoot, "pet.json"), filepath.Join(testAssetsRoot, "pets", testPetID))
	if err != nil {
		t.Fatal(err)
	}
	return manifest
}

func TestAutoDiscoverAndMergedPetManifest(t *testing.T) {
	profile, _, err := loadRuntimeProfile("", testAssetsRoot, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(profile.ActivePets) != 1 {
		t.Fatalf("expected one default discovered pet group, got %d", len(profile.ActivePets))
	}
	if profile.ActivePets[0].PetID != testPetID {
		t.Fatalf("expected default pet %q, got %q", testPetID, profile.ActivePets[0].PetID)
	}

	allProfile, _, err := loadRuntimeProfile("", testAssetsRoot, "all")
	if err != nil {
		t.Fatal(err)
	}
	if len(allProfile.ActivePets) < 1 {
		t.Fatalf("expected at least 1 discovered pet group, got %d", len(allProfile.ActivePets))
	}

	manifest := testManifest(t)
	if manifest.ID != testPetID {
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

func TestUnknownPetSelectionReturnsAvailablePets(t *testing.T) {
	_, _, err := loadRuntimeProfile("", testAssetsRoot, "missing-pet")
	if err == nil {
		t.Fatal("expected missing pet selection to fail")
	}
	msg := err.Error()
	if !strings.Contains(msg, "missing-pet") || !strings.Contains(msg, testPetID) || !strings.Contains(msg, "available=") {
		t.Fatalf("error should include missing id and available pets, got %q", msg)
	}
}

func TestManifestDefaultsAndBlacklistAreIdempotent(t *testing.T) {
	manifest := testManifest(t)
	if manifest.Scale <= 0 {
		t.Fatalf("scale default was not normalized")
	}
	if manifest.Columns <= 0 {
		t.Fatalf("columns default was not normalized")
	}
	if manifest.Motion.AutoRoamChance <= 0 {
		t.Fatalf("motion.auto_roam_chance default was not normalized")
	}
	seen := map[string]bool{}
	for _, item := range manifest.ActBlacklist {
		if seen[item] {
			t.Fatalf("duplicate blacklist item %q", item)
		}
		seen[item] = true
	}
}

func TestLoadSpriteStorePerPet(t *testing.T) {
	manifest := testManifest(t)
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
	manifest := testManifest(t)
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
