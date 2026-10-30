// mkicon draws the app's icon: a gradient square with a chat bubble in it.
// It writes resources/icon.png, the 1024 square mygo build puts in the
// bundle.
//
//	go run ./tools/mkicon
package main

import (
	"image"
	"image/color"
	"image/png"
	"log"
	"math"
	"os"
	"path/filepath"

	xdraw "golang.org/x/image/draw"
)

// px is the square the icon is drawn at, twice the 1024 of the file, so
// that its edges are smooth when scaled down.
const px = 2048

func main() {
	img := image.NewRGBA(image.Rect(0, 0, px, px))

	// The rounded square of the app icons, filled with a gradient.
	top := color.RGBA{0x5B, 0x8D, 0xEF, 0xFF}
	bottom := color.RGBA{0x8A, 0x63, 0xE8, 0xFF}
	cx, cy := float64(px)/2, float64(px)/2
	half := float64(px)/2 - float64(px)*0.09
	for y := 0; y < px; y++ {
		row := mix(top, bottom, float64(y)/float64(px))
		for x := 0; x < px; x++ {
			d := sdRoundRect(float64(x)+0.5-cx, float64(y)+0.5-cy, half, half, float64(px)*0.22)
			if d < 1 {
				paint(img, x, y, row, coverage(d))
			}
		}
	}

	// The white chat bubble, its tail at the lower left, and the dots in
	// its color. The mark stays clear of the tile's edge and, above all,
	// of its rounded corners: a tail that reaches them loses the ring of
	// color that makes the corner, and the icon looks cut off where it
	// shows small.
	bw, bh := float64(px)*0.275, float64(px)*0.215 // half sizes
	by := float64(px) * 0.006                      // the bubble's center, below the tile's
	radius := bh * 0.30
	white := color.RGBA{0xFF, 0xFF, 0xFF, 0xFF}
	// The tail is a triangle: two of its corners sit inside the bubble, so
	// what shows past its left and bottom edges is a sharp point, as a chat
	// bubble's tail is. It reaches well past both edges — a shorter tail
	// merges with the bubble's rounded corner, and the icon then reads as cut
	// off where it shows small, in the Dock's 36 to 64 dots.
	// The tail hangs below the bubble as the reference icon's does: its left
	// side drops straight down, and its right side slopes up to the bubble's
	// bottom, so the point aims down, not out. The corner and the point share
	// an x, which is what keeps that side straight.
	tailX := -bw * 0.72                    // the side, and the point
	tailAX, tailAY := tailX, by+bh*0.80    // inside, above the bubble's bottom
	tailBX, tailBY := -bw*0.05, by+bh*0.95 // inside, on the bubble's bottom
	tipX, tipY := tailX, by+bh*1.45        // the point, below the bubble
	for y := 0; y < px; y++ {
		for x := 0; x < px; x++ {
			dx, dy := float64(x)+0.5-cx, float64(y)+0.5-cy
			d := math.Min(
				sdRoundRect(dx, dy-by, bw, bh, radius),
				sdTriangle(dx, dy, tailAX, tailAY, tailBX, tailBY, tipX, tipY),
			)
			if d < 1 {
				paint(img, x, y, white, coverage(d))
			}
		}
	}
	dot := color.RGBA{0x7A, 0x78, 0xEB, 0xFF}
	for _, dx := range []float64{-bw * 0.40, 0, bw * 0.40} {
		fillCircle(img, cx+dx, cy+by, bh*0.135, dot)
	}

	// The white of the mark must keep away from the tile's edge, the
	// rounded corners most of all, or the icon reads as cut off there.
	checkClearance(img, cx, cy, half, float64(px)*0.22)

	// Scale down to the 1024 the file holds.
	const out = 1024
	dst := image.NewRGBA(image.Rect(0, 0, out, out))
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), img, img.Bounds(), xdraw.Over, nil)

	if err := os.MkdirAll("resources", 0o755); err != nil {
		log.Fatal(err)
	}
	path := filepath.Join("resources", "icon.png")
	f, err := os.Create(path)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, dst); err != nil {
		log.Fatal(err)
	}
	log.Printf("wrote %s, %dx%d", path, out, out)
}

// checkClearance reports how far the white of the mark stays inside the
// tile, and stops the drawing when it comes too close to its edge: the
// ring of color the rounded corners leave would be too thin to see.
func checkClearance(img *image.RGBA, cx, cy, half, radius float64) {
	closest, at := math.Inf(1), image.Point{}
	for y := 0; y < px; y += 1 {
		for x := 0; x < px; x += 1 {
			r, g, b, a := img.RGBAAt(x, y).RGBA()
			if a>>8 < 8 || r>>8 < 235 || g>>8 < 235 || b>>8 < 235 {
				continue
			}
			// The distance to the tile's edge, positive inside it.
			d := -sdRoundRect(float64(x)+0.5-cx, float64(y)+0.5-cy, half, half, radius)
			if d < closest {
				closest, at = d, image.Pt(x, y)
			}
		}
	}
	const want = 0.055 * px // 5.5% of the icon, about 2px of a 36px one
	log.Printf("the mark keeps %.0f px of the tile's edge (its 1024 share: %.0f), at %v",
		closest, closest/2, at)
	if closest < want {
		log.Fatalf("the mark comes within %.0f px of the tile's edge, want %.0f: move it in",
			closest, want)
	}
}

// sdRoundRect is the signed distance to a rounded rectangle centered on
// (0, 0) of half sizes hw and hh, negative inside it.
func sdRoundRect(x, y, hw, hh, r float64) float64 {
	qx, qy := math.Abs(x)-hw+r, math.Abs(y)-hh+r
	outside := math.Hypot(math.Max(qx, 0), math.Max(qy, 0))
	inside := math.Min(math.Max(qx, qy), 0)
	return outside + inside - r
}

// sdRotRect is sdRoundRect of a rounded rectangle turned by angle around
// its center at (cx, cy).
// sdTriangle is the distance to the inside of the triangle a, b, c, positive
// within: the furthest of the three edges' half planes. It is exact inside
// and close enough outside for the one pixel of antialiasing the icon uses.
func sdTriangle(px, py float64, ax, ay, bx, by, cx, cy float64) float64 {
	d1 := halfPlane(px, py, ax, ay, bx, by, cx, cy)
	d2 := halfPlane(px, py, bx, by, cx, cy, ax, ay)
	d3 := halfPlane(px, py, cx, cy, ax, ay, bx, by)
	// The three are positive inside: the nearest, negated, is the distance to
	// the triangle, positive outside, as the other shapes' are. (It is the
	// min, not the max: a point on one edge is inside the other two.)
	return -math.Min(d1, math.Min(d2, d3))
}

// halfPlane is the signed distance from (px,py) to the line a-b, positive on
// the side c is on.
func halfPlane(px, py, ax, ay, bx, by, cx, cy float64) float64 {
	nx, ny := by-ay, ax-bx
	length := math.Hypot(nx, ny)
	if length == 0 {
		return 0
	}
	nx, ny = nx/length, ny/length
	d := (px-ax)*nx + (py-ay)*ny
	if (cx-ax)*nx+(cy-ay)*ny < 0 {
		d = -d
	}
	return d
}

func sdRotRect(x, y, cx, cy, hw, hh, r, angle float64) float64 {
	dx, dy := x-cx, y-cy
	c, s := math.Cos(-angle), math.Sin(-angle)
	return sdRoundRect(dx*c-dy*s, dx*s+dy*c, hw, hh, r)
}

// coverage turns a signed distance into how much of the pixel the shape
// covers.
func coverage(d float64) float64 {
	return math.Max(0, math.Min(1, 0.5-d))
}

// fillCircle paints the disc at (cx, cy).
func fillCircle(img *image.RGBA, cx, cy, r float64, c color.RGBA) {
	for y := int(cy - r - 1); y <= int(cy+r+1); y++ {
		for x := int(cx - r - 1); x <= int(cx+r+1); x++ {
			if !image.Pt(x, y).In(img.Bounds()) {
				continue
			}
			if d := math.Hypot(float64(x)+0.5-cx, float64(y)+0.5-cy) - r; d < 1 {
				paint(img, x, y, c, coverage(d))
			}
		}
	}
}

// paint draws c over the pixel at (x, y), with coverage a, in the
// premultiplied alpha the image holds.
func paint(img *image.RGBA, x, y int, c color.RGBA, a float64) {
	if a <= 0 {
		return
	}
	sa := float64(c.A) / 255 * a
	d := img.RGBAAt(x, y)
	dr := float64(d.R) / 255
	dg := float64(d.G) / 255
	db := float64(d.B) / 255
	keep := 1 - sa
	outA := sa + float64(d.A)/255*keep
	if outA <= 0 {
		return
	}
	img.SetRGBA(x, y, color.RGBA{
		R: uint8(math.Round((float64(c.R)/255*sa + dr*keep) * 255)),
		G: uint8(math.Round((float64(c.G)/255*sa + dg*keep) * 255)),
		B: uint8(math.Round((float64(c.B)/255*sa + db*keep) * 255)),
		A: uint8(math.Round(outA * 255)),
	})
}

// mix blends two colors by t, 0 for a and 1 for b.
func mix(a, b color.RGBA, t float64) color.RGBA {
	return color.RGBA{
		R: uint8(float64(a.R) + (float64(b.R)-float64(a.R))*t),
		G: uint8(float64(a.G) + (float64(b.G)-float64(a.G))*t),
		B: uint8(float64(a.B) + (float64(b.B)-float64(a.B))*t),
		A: 0xFF,
	}
}
