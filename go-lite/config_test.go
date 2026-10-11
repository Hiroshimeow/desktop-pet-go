package main

import (
	"encoding/json"
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

// discoveryFixture deliberately creates pets out of lexical order and includes
// non-pets. Discovery tests must not depend on optional user-installed assets.
func discoveryFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	base := PetManifest{
		Schema: 1, FrameWidth: 2, FrameHeight: 2, Columns: 2,
		DefaultAnimation: "idle", AnimationDir: "animations",
		ActBlacklist: []string{"hidden*"},
		Animations: map[string]AnimationDef{
			"idle": {File: "idle.png", FPS: 4, Loop: true, Kind: "state"},
		},
	}
	data, err := json.Marshal(base)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "pet.json"), data, 0644); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"zeta", "alpha"} {
		dir := filepath.Join(root, "pets", id, "animations")
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		for _, name := range []string{"idle", "walk", "run", "cry", "angry", "hidden_test"} {
			writeTestPNG(t, filepath.Join(dir, name+".png"), 4, 2)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "pets", "alpha", "pet.json"), []byte(`{"name":"Alpha override","scale":0.75,"animations":{"idle":{"fps":9}}}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "pets", "not-a-pet"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "pets", "not-a-directory"), []byte("ignored"), 0644); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestAutoDiscoverAndMergedPetManifest(t *testing.T) {
	assetsRoot := discoveryFixture(t)
	profile, profileDir, err := loadRuntimeProfile("", assetsRoot, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(profile.ActivePets) != 1 {
		t.Fatalf("expected one default discovered pet group, got %d", len(profile.ActivePets))
	}
	if profile.ActivePets[0].PetID != "alpha" {
		t.Fatalf("expected lexical first pet alpha, got %q", profile.ActivePets[0].PetID)
	}
	if profileDir != "." || profile.Schema != 2 {
		t.Fatalf("unexpected discovered profile: dir=%q schema=%d", profileDir, profile.Schema)
	}

	allProfile, _, err := loadRuntimeProfile("", assetsRoot, "all")
	if err != nil {
		t.Fatal(err)
	}
	if len(allProfile.ActivePets) != 2 {
		t.Fatalf("expected exactly two discovered pets, got %+v", allProfile.ActivePets)
	}
	for i, id := range []string{"alpha", "zeta"} {
		if allProfile.ActivePets[i].PetID != id {
			t.Fatalf("pet %d: expected %q, got %+v", i, id, allProfile.ActivePets[i])
		}
	}
	selected, _, err := loadRuntimeProfile("", assetsRoot, "zeta,alpha,zeta")
	if err != nil {
		t.Fatal(err)
	}
	if len(selected.ActivePets) != 2 || selected.ActivePets[0].PetID != "zeta" || selected.ActivePets[1].PetID != "alpha" {
		t.Fatalf("explicit selection must preserve order and deduplicate: %+v", selected.ActivePets)
	}

	manifest, err := LoadPetManifestMerged(filepath.Join(assetsRoot, "pet.json"), filepath.Join(assetsRoot, "pets", "alpha"))
	if err != nil {
		t.Fatal(err)
	}
	if manifest.ID != "alpha" || manifest.Name != "Alpha override" || manifest.Scale != 0.75 {
		t.Fatalf("pet-local overrides not merged: %+v", manifest)
	}
	if idle := manifest.Animations["idle"]; idle.FPS != 9 || !idle.Loop || idle.Kind != "state" {
		t.Fatalf("default animation metadata not merged with pet override: %+v", idle)
	}
	if _, ok := manifest.Animations["hidden_test"]; ok {
		t.Fatal("blacklisted animation should not be discovered")
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
	_, _, err := loadRuntimeProfile("", discoveryFixture(t), "missing-pet")
	if err == nil {
		t.Fatal("expected missing pet selection to fail")
	}
	msg := err.Error()
	if !strings.Contains(msg, "missing-pet") || !strings.Contains(msg, "available=alpha,zeta") {
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
