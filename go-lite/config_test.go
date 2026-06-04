package main

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var testAssetsRoot = filepath.Join("..", "assets")

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
	p.Update(0.2, 1000, 128)
	if p.X != oldX {
		t.Fatalf("drag emotion should not move by itself")
	}
	if manifest.Animations[p.Animation].Locomotion {
		t.Fatalf("drag should use emotion/state, got %s", p.Animation)
	}
}

func TestLoadSpriteStoreSkipsInvalidOptionalAnimation(t *testing.T) {
	dir := t.TempDir()
	animDir := filepath.Join(dir, "animations")
	if err := os.MkdirAll(animDir, 0755); err != nil {
		t.Fatal(err)
	}
	writeTestPNG(t, filepath.Join(animDir, "idle.png"), 512, 256)
	writeTestPNG(t, filepath.Join(animDir, "bad.png"), 512, 300)
	manifest := PetManifest{
		ID:               "skip-test",
		Name:             "skip-test",
		Scale:            1,
		FrameWidth:       256,
		FrameHeight:      256,
		Columns:          2,
		DefaultAnimation: "idle",
		AnimationDir:     "animations",
		BaseDir:          dir,
		Interactions: map[string]InteractionAction{
			"left_click": {Animation: "bad", DurationMS: 500},
		},
		Animations: map[string]AnimationDef{
			"idle": {File: "idle.png", FPS: 4, Frames: 2},
			"bad":  {File: "bad.png", FPS: 4, Frames: 2},
		},
	}
	store, err := LoadSpriteStore(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := store.Animations["bad"]; ok {
		t.Fatal("invalid optional animation should be skipped")
	}
	if _, ok := store.Manifest.Animations["bad"]; ok {
		t.Fatal("invalid optional animation should be removed from manifest")
	}
	if _, ok := store.Manifest.Interactions["left_click"]; ok {
		t.Fatal("interaction pointing to skipped animation should be sanitized")
	}
}

func TestLoadSpriteStoreFailsInvalidDefaultAnimation(t *testing.T) {
	dir := t.TempDir()
	animDir := filepath.Join(dir, "animations")
	if err := os.MkdirAll(animDir, 0755); err != nil {
		t.Fatal(err)
	}
	writeTestPNG(t, filepath.Join(animDir, "idle.png"), 512, 300)
	manifest := PetManifest{
		ID:               "bad-default-test",
		Name:             "bad-default-test",
		Scale:            1,
		FrameWidth:       256,
		FrameHeight:      256,
		Columns:          2,
		DefaultAnimation: "idle",
		AnimationDir:     "animations",
		BaseDir:          dir,
		Animations: map[string]AnimationDef{
			"idle": {File: "idle.png", FPS: 4, Frames: 2},
		},
	}
	_, err := LoadSpriteStore(manifest)
	if err == nil || !strings.Contains(err.Error(), "default animation") {
		t.Fatalf("expected invalid default animation error, got %v", err)
	}
}

func writeTestPNG(t *testing.T, path string, w, h int) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, image.NewNRGBA(image.Rect(0, 0, w, h))); err != nil {
		t.Fatal(err)
	}
}
