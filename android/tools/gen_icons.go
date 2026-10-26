// gen_icons.go draws the VOM launcher icon at every density.  It is only a
// source asset generator; the PNGs it writes are committed, no image library
// is needed on the build host.
package main

import (
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
)

var densities = map[string]int{
	"mipmap-mdpi":    48,
	"mipmap-hdpi":    72,
	"mipmap-xhdpi":   96,
	"mipmap-xxhdpi":  144,
	"mipmap-xxxhdpi": 192,
}

func main() {
	root := "res"
	if len(os.Args) > 1 {
		root = os.Args[1]
	}
	for dir, size := range densities {
		img := draw(size)
		path := filepath.Join(root, dir, "ic_launcher.png")
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			panic(err)
		}
		f, err := os.Create(path)
		if err != nil {
			panic(err)
		}
		if err := png.Encode(f, img); err != nil {
			panic(err)
		}
		f.Close()
	}
}

func draw(size int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	r := float64(size) * 0.22
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			if !insideRounded(float64(x)+0.5, float64(y)+0.5, float64(size), float64(size), r) {
				continue
			}
			t := (float64(x) + float64(y)) / (2 * float64(size))
			img.Set(x, y, lerp(color.RGBA{0x67, 0x50, 0xA4, 255}, color.RGBA{0x9A, 0x82, 0xDB, 255}, t))
		}
	}
	// phone body
	left := float64(size) * 0.31
	right := float64(size) * 0.69
	top := float64(size) * 0.20
	bottom := float64(size) * 0.80
	radius := float64(size) * 0.09
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			fx, fy := float64(x)+0.5, float64(y)+0.5
			if insideRoundedRect(fx, fy, left, top, right, bottom, radius) &&
				!insideRoundedRect(fx, fy, left+float64(size)*0.035, top+float64(size)*0.035,
					right-float64(size)*0.035, bottom-float64(size)*0.035, radius*0.7) {
				img.Set(x, y, color.RGBA{255, 255, 255, 255})
			}
		}
	}
	// play triangle pointing right
	cx := float64(size) * 0.425
	cy := float64(size) * 0.50
	h := float64(size) * 0.14
	w := float64(size) * 0.20
	for y := int(cy - h); y <= int(cy+h); y++ {
		fy := (float64(y) + 0.5 - cy) / h
		if fy < -1 || fy > 1 {
			continue
		}
		t := 1 - abs(fy)
		x0 := int(cx)
		x1 := int(cx + w*t)
		for x := x0; x <= x1; x++ {
			img.Set(x, y, color.RGBA{0x38, 0x1E, 0x72, 255})
		}
	}
	return img
}

func insideRounded(x, y, w, h, r float64) bool {
	if x < r && y < r {
		return dist(x, y, r, r) <= r
	}
	if x > w-r && y < r {
		return dist(x, y, w-r, r) <= r
	}
	if x < r && y > h-r {
		return dist(x, y, r, h-r) <= r
	}
	if x > w-r && y > h-r {
		return dist(x, y, w-r, h-r) <= r
	}
	return x >= 0 && y >= 0 && x <= w && y <= h
}

func insideRoundedRect(x, y, l, t, r, b, rad float64) bool {
	if x < l || x > r || y < t || y > b {
		return false
	}
	if x < l+rad && y < t+rad {
		return dist(x, y, l+rad, t+rad) <= rad
	}
	if x > r-rad && y < t+rad {
		return dist(x, y, r-rad, t+rad) <= rad
	}
	if x < l+rad && y > b-rad {
		return dist(x, y, l+rad, b-rad) <= rad
	}
	if x > r-rad && y > b-rad {
		return dist(x, y, r-rad, b-rad) <= rad
	}
	return true
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

func dist(x, y, cx, cy float64) float64 {
	dx, dy := x-cx, y-cy
	return math.Sqrt(dx*dx + dy*dy)
}

func lerp(a, b color.RGBA, t float64) color.RGBA {
	return color.RGBA{
		R: uint8(float64(a.R) + (float64(b.R)-float64(a.R))*t),
		G: uint8(float64(a.G) + (float64(b.G)-float64(a.G))*t),
		B: uint8(float64(a.B) + (float64(b.B)-float64(a.B))*t),
		A: 255,
	}
}
