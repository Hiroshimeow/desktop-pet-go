//go:build tools

package main

import (
	"flag"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func main() {
	framesDir := flag.String("frames", "", "folder containing ordered PNG frames")
	outPath := flag.String("out", "", "output horizontal PNG strip")
	frameWidth := flag.Int("width", 0, "output frame width in pixels")
	frameHeight := flag.Int("height", 0, "output frame height in pixels")
	baselineY := flag.Int("baseline-y", 0, "output bottom-center feet baseline in pixels")
	flag.Parse()

	if err := packFrames(*framesDir, *outPath, *frameWidth, *frameHeight, *baselineY); err != nil {
		fmt.Fprintln(os.Stderr, "pack frames:", err)
		os.Exit(1)
	}
}

func packFrames(framesDir, outPath string, frameWidth, frameHeight, baselineY int) error {
	if framesDir == "" || outPath == "" {
		return fmt.Errorf("-frames and -out are required")
	}
	if frameWidth <= 0 || frameHeight <= 0 {
		return fmt.Errorf("frame size must be positive")
	}
	if baselineY <= 0 || baselineY > frameHeight {
		return fmt.Errorf("baseline-y must be within 1..%d", frameHeight)
	}

	entries, err := os.ReadDir(framesDir)
	if err != nil {
		return fmt.Errorf("read frames folder: %w", err)
	}
	outAbs, err := filepath.Abs(outPath)
	if err != nil {
		return fmt.Errorf("resolve output path: %w", err)
	}
	var paths []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".png") {
			continue
		}
		path := filepath.Join(framesDir, entry.Name())
		pathAbs, err := filepath.Abs(path)
		if err != nil {
			return fmt.Errorf("resolve frame %s: %w", entry.Name(), err)
		}
		if filepath.Clean(pathAbs) == filepath.Clean(outAbs) {
			continue
		}
		paths = append(paths, path)
	}
	sort.Strings(paths)
	if len(paths) == 0 {
		return fmt.Errorf("frames folder contains no PNG files")
	}

	strip := image.NewNRGBA(image.Rect(0, 0, frameWidth*len(paths), frameHeight))
	for i, path := range paths {
		img, err := decodePNG(path)
		if err != nil {
			return err
		}
		bounds, ok := visibleAlphaBounds(img)
		if !ok {
			return fmt.Errorf("frame %s is fully transparent", filepath.Base(path))
		}
		left := frameWidth/2 - bounds.Dx()/2
		top := baselineY - bounds.Dy()
		if left < 0 || top < 0 || left+bounds.Dx() > frameWidth || baselineY > frameHeight {
			return fmt.Errorf("frame %s visible bounds %dx%d do not fit %dx%d at baseline-y=%d", filepath.Base(path), bounds.Dx(), bounds.Dy(), frameWidth, frameHeight, baselineY)
		}
		dst := image.Rect(i*frameWidth+left, top, i*frameWidth+left+bounds.Dx(), baselineY)
		draw.Draw(strip, dst, img, bounds.Min, draw.Src)
	}

	if err := os.MkdirAll(filepath.Dir(outAbs), 0o755); err != nil {
		return fmt.Errorf("create output folder: %w", err)
	}
	f, err := os.Create(outAbs)
	if err != nil {
		return fmt.Errorf("create output: %w", err)
	}
	if err := png.Encode(f, strip); err != nil {
		f.Close()
		return fmt.Errorf("encode output: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close output: %w", err)
	}
	fmt.Printf("packed %d frame(s) -> %s (%dx%d each, baseline-y=%d)\n", len(paths), outAbs, frameWidth, frameHeight, baselineY)
	return nil
}

func decodePNG(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open frame %s: %w", filepath.Base(path), err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("decode frame %s: %w", filepath.Base(path), err)
	}
	return img, nil
}

func visibleAlphaBounds(img image.Image) (image.Rectangle, bool) {
	b := img.Bounds()
	minX, minY := b.Max.X, b.Max.Y
	maxX, maxY := b.Min.X, b.Min.Y
	found := false
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			_, _, _, a := img.At(x, y).RGBA()
			if a == 0 {
				continue
			}
			found = true
			if x < minX {
				minX = x
			}
			if y < minY {
				minY = y
			}
			if x+1 > maxX {
				maxX = x + 1
			}
			if y+1 > maxY {
				maxY = y + 1
			}
		}
	}
	if !found {
		return image.Rectangle{}, false
	}
	return image.Rect(minX, minY, maxX, maxY), true
}
