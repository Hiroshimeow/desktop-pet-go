//go:build tools

package main

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
)

const toolFrameW = 256
const toolFrameH = 256
const toolCols = 5
const toolRows = 8

var rowsFromAnimation1 = []string{
	"idle.png",
	"walk.png",
	"run.png",
	"happy.png",
	"cry.png",
	"angry.png",
	"wave.png",
	"sleepy.png",
}

var rowsFromAnimation2 = []string{
	"surprised.png",
	"shy.png",
	"thinking.png",
	"cheer.png",
	"scared.png",
	"dizzy.png",
	"dance.png",
	"sit_idle.png",
}

func main() {
	if err := split("..\\assets\\animation1.png", "..\\assets\\animations", rowsFromAnimation1); err != nil {
		panic(err)
	}
	if err := split("..\\assets\\animation2.png", "..\\assets\\animations", rowsFromAnimation2); err != nil {
		panic(err)
	}
}

func split(srcPath, outDir string, names []string) error {
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return err
	}
	file, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer file.Close()
	img, _, err := image.Decode(file)
	if err != nil {
		return err
	}
	b := img.Bounds()
	cellW := float64(b.Dx()) / float64(toolCols)
	cellH := float64(b.Dy()) / float64(toolRows)
	for row, name := range names {
		strip := image.NewNRGBA(image.Rect(0, 0, toolFrameW*toolCols, toolFrameH))
		for col := 0; col < toolCols; col++ {
			left := int(float64(col)*cellW + 0.5)
			top := int(float64(row)*cellH + 0.5)
			right := int(float64(col+1)*cellW + 0.5)
			bottom := int(float64(row+1)*cellH + 0.5)
			frame := image.NewNRGBA(image.Rect(0, 0, toolFrameW, toolFrameH))
			dst := image.Rect(col*toolFrameW, 0, (col+1)*toolFrameW, toolFrameH)
			scaleNearestTransparent(strip, dst, img, image.Rect(left, top, right, bottom))
			_ = frame
		}
		outPath := filepath.Join(outDir, name)
		out, err := os.Create(outPath)
		if err != nil {
			return err
		}
		if err := png.Encode(out, strip); err != nil {
			out.Close()
			return err
		}
		out.Close()
		fmt.Println(outPath)
	}
	return nil
}

func scaleNearestTransparent(dst draw.Image, dr image.Rectangle, src image.Image, sr image.Rectangle) {
	for y := dr.Min.Y; y < dr.Max.Y; y++ {
		sy := sr.Min.Y + (y-dr.Min.Y)*sr.Dy()/dr.Dy()
		for x := dr.Min.X; x < dr.Max.X; x++ {
			sx := sr.Min.X + (x-dr.Min.X)*sr.Dx()/dr.Dx()
			r, g, b, a := src.At(sx, sy).RGBA()
			r8, g8, b8, a8 := uint8(r>>8), uint8(g>>8), uint8(b>>8), uint8(a>>8)
			if r8 > 238 && g8 > 238 && b8 > 238 {
				a8 = 0
			}
			dst.Set(x, y, color.NRGBA{R: r8, G: g8, B: b8, A: a8})
		}
	}
}
