package main

import (
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"log"
	"os"
)

type AnimationStrip struct {
	Name   string
	Def    AnimationDef
	Frames int
	Image  *image.NRGBA
}

type SpriteStore struct {
	Manifest   PetManifest
	Animations map[string]*AnimationStrip
}

func LoadSpriteStore(manifest PetManifest) (*SpriteStore, error) {
	store := &SpriteStore{Manifest: manifest, Animations: map[string]*AnimationStrip{}}
	loadedDefs := make(map[string]AnimationDef, len(manifest.Animations))
	var skipped []string
	for name, def := range manifest.Animations {
		path := manifest.AnimationPath(def)
		strip, err := loadStrip(name, def, path, manifest)
		if err != nil {
			if name == manifest.DefaultAnimation {
				return nil, fmt.Errorf("default animation %q failed to load: %w", name, err)
			}
			log.Printf("skip invalid optional animation pet=%s name=%s file=%s err=%v", manifest.ID, name, path, err)
			skipped = append(skipped, name)
			continue
		}
		def.Frames = strip.Frames
		loadedDefs[name] = def
		store.Animations[name] = strip
	}
	if len(store.Animations) == 0 {
		return nil, fmt.Errorf("pet %s has no loadable animations", manifest.ID)
	}
	manifest.Animations = loadedDefs
	sanitizeInteractions(&manifest)
	store.Manifest = manifest
	if len(skipped) > 0 {
		log.Printf("pet=%s skipped %d invalid optional animations: %v", manifest.ID, len(skipped), skipped)
	}
	return store, nil
}

func loadStrip(name string, def AnimationDef, path string, manifest PetManifest) (*AnimationStrip, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open animation %s: %w", path, err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("decode animation %s: %w", path, err)
	}
	b := img.Bounds()
	frames, err := detectFrameCount(name, def, path, b.Dx(), b.Dy(), manifest)
	if err != nil {
		return nil, err
	}
	wantW := manifest.FrameWidth * frames
	wantH := manifest.FrameHeight
	out := image.NewNRGBA(image.Rect(0, 0, wantW, wantH))
	draw.Draw(out, out.Bounds(), img, b.Min, draw.Src)
	def.Frames = frames
	return &AnimationStrip{Name: name, Def: def, Frames: frames, Image: out}, nil
}

func (s *SpriteStore) FrameRect(animation string, frame int) (*AnimationStrip, int, int, int, int, error) {
	strip, ok := s.Animations[animation]
	if !ok {
		return nil, 0, 0, 0, 0, fmt.Errorf("unknown animation %q for pet %s", animation, s.Manifest.ID)
	}
	frames := strip.Frames
	if frames <= 0 {
		frames = frameCountOf(s.Manifest, strip.Def)
	}
	col := frame % frames
	if col < 0 {
		col += frames
	}
	return strip, col * s.Manifest.FrameWidth, 0, s.Manifest.FrameWidth, s.Manifest.FrameHeight, nil
}

func detectFrameCount(name string, def AnimationDef, path string, imageW, imageH int, manifest PetManifest) (int, error) {
	if imageH != manifest.FrameHeight {
		return 0, fmt.Errorf("animation %s height mismatch: got %d want %d", path, imageH, manifest.FrameHeight)
	}
	if manifest.FrameWidth <= 0 {
		return 0, fmt.Errorf("animation %s invalid frame_width=%d", path, manifest.FrameWidth)
	}
	if imageW%manifest.FrameWidth == 0 {
		detected := imageW / manifest.FrameWidth
		if detected <= 0 {
			return 0, fmt.Errorf("animation %s has invalid detected frame count %d", path, detected)
		}
		configured := frameCountOf(manifest, def)
		if configured > 0 && configured != detected {
			fmt.Fprintf(os.Stderr, "warning: animation %s configured frames=%d but PNG width=%d/frame_width=%d gives frames=%d; using detected frames\n", name, configured, imageW, manifest.FrameWidth, detected)
		}
		return detected, nil
	}
	configured := frameCountOf(manifest, def)
	wantW := manifest.FrameWidth * configured
	return 0, fmt.Errorf("animation %s width %d is not divisible by frame_width %d. Configured frames=%d implies expected width=%d; fix the PNG width or update frames/columns in pet.json", path, imageW, manifest.FrameWidth, configured, wantW)
}
