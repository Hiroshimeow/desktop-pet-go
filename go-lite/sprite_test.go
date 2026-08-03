package main

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadSpriteStoreDetectsArbitraryFrameCount(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	strip := image.NewNRGBA(image.Rect(0, 0, 6, 2))
	for y := 0; y < 2; y++ {
		for x := 0; x < 6; x++ {
			strip.SetNRGBA(x, y, color.NRGBA{R: uint8(20 + x), G: uint8(40 + y), B: 60, A: 255})
		}
	}
	writeSpriteTestPNG(t, filepath.Join(dir, "three.png"), strip)

	manifest := PetManifest{
		ID:               "test-pet",
		FrameWidth:       2,
		FrameHeight:      2,
		Columns:          5,
		DefaultAnimation: "idle",
		AnimationDir:     ".",
		BaseDir:          dir,
		Animations: map[string]AnimationDef{
			"idle": {File: "three.png", FPS: 5, Frames: 5},
		},
	}

	store, err := LoadSpriteStore(manifest)
	if err != nil {
		t.Fatalf("LoadSpriteStore: %v", err)
	}
	if got := store.Manifest.Animations["idle"].Frames; got != 3 {
		t.Fatalf("detected frames = %d, want 3", got)
	}
}

func writeSpriteTestPNG(t *testing.T, path string, img image.Image) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create PNG: %v", err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatalf("encode PNG: %v", err)
	}
}

func TestRenderFrameCachesScaledFlippedBGRA(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	strip := image.NewNRGBA(image.Rect(0, 0, 2, 1))
	strip.SetNRGBA(0, 0, color.NRGBA{R: 10, G: 20, B: 30, A: 40})
	strip.SetNRGBA(1, 0, color.NRGBA{R: 50, G: 60, B: 70, A: 80})
	writeSpriteTestPNG(t, filepath.Join(dir, "idle.png"), strip)

	manifest := PetManifest{
		ID:               "test-pet",
		FrameWidth:       2,
		FrameHeight:      1,
		Columns:          1,
		DefaultAnimation: "idle",
		AnimationDir:     ".",
		BaseDir:          dir,
		Animations: map[string]AnimationDef{
			"idle": {File: "idle.png", FPS: 5, Frames: 1},
		},
	}
	store, err := LoadSpriteStore(manifest)
	if err != nil {
		t.Fatalf("LoadSpriteStore: %v", err)
	}

	got, err := store.RenderFrame("idle", 0, 4, 1, false)
	if err != nil {
		t.Fatalf("RenderFrame: %v", err)
	}
	want := []byte{
		30, 20, 10, 40,
		30, 20, 10, 40,
		70, 60, 50, 80,
		70, 60, 50, 80,
	}
	if !bytes.Equal(got.BGRA, want) {
		t.Fatalf("unflipped BGRA = %v, want %v", got.BGRA, want)
	}

	again, err := store.RenderFrame("idle", 0, 4, 1, false)
	if err != nil {
		t.Fatalf("RenderFrame cache hit: %v", err)
	}
	if again != got {
		t.Fatal("same render key did not reuse cached render frame")
	}
	beforeFlip := append([]byte(nil), got.BGRA...)

	flipped, err := store.RenderFrame("idle", 0, 4, 1, true)
	if err != nil {
		t.Fatalf("RenderFrame flipped: %v", err)
	}
	wantFlipped := []byte{
		70, 60, 50, 80,
		70, 60, 50, 80,
		30, 20, 10, 40,
		30, 20, 10, 40,
	}
	if !bytes.Equal(flipped.BGRA, wantFlipped) {
		t.Fatalf("flipped BGRA = %v, want %v", flipped.BGRA, wantFlipped)
	}
	if !bytes.Equal(got.BGRA, beforeFlip) {
		t.Fatal("building flipped cache entry mutated the unflipped cached frame")
	}
}

func TestRenderDecisionSkipsUnchangedAndCopiesOnlyVisualChanges(t *testing.T) {
	t.Parallel()

	current := visibleRenderState{Animation: "idle", Frame: 0, X: 10, Y: 20, Flip: false, Width: 128, Height: 128}
	copyPixels, updateWindow := decideRender(nil, current)
	if !copyPixels || !updateWindow {
		t.Fatalf("first draw = copy %v update %v, want true/true", copyPixels, updateWindow)
	}

	previous := current
	copyPixels, updateWindow = decideRender(&previous, current)
	if copyPixels || updateWindow {
		t.Fatalf("unchanged draw = copy %v update %v, want false/false", copyPixels, updateWindow)
	}

	positionOnly := current
	positionOnly.X++
	copyPixels, updateWindow = decideRender(&previous, positionOnly)
	if copyPixels || !updateWindow {
		t.Fatalf("position-only draw = copy %v update %v, want false/true", copyPixels, updateWindow)
	}

	visualChanges := []visibleRenderState{
		func() visibleRenderState { s := current; s.Frame++; return s }(),
		func() visibleRenderState { s := current; s.Animation = "walk"; return s }(),
		func() visibleRenderState { s := current; s.Flip = true; return s }(),
		func() visibleRenderState { s := current; s.Width++; return s }(),
	}
	for _, changed := range visualChanges {
		copyPixels, updateWindow = decideRender(&previous, changed)
		if !copyPixels || !updateWindow {
			t.Fatalf("visual change %+v = copy %v update %v, want true/true", changed, copyPixels, updateWindow)
		}
	}
}
