//go:build tooliconpreview

// Pixel checks need an Ebiten window, so they stay behind this tag.
// go test -tags tooliconpreview -run 'TestToolGlyph|TestBranchMark'

package app

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

type toolIconGame struct {
	m    *testing.M
	code int
}

func (g *toolIconGame) Update() error {
	g.code = g.m.Run()
	return ebiten.Termination
}

func (*toolIconGame) Draw(*ebiten.Image) {}

func (*toolIconGame) Layout(int, int) (int, int) { return 320, 240 }

func TestMain(m *testing.M) {
	g := &toolIconGame{m: m, code: 1}
	if err := ebiten.RunGame(g); err != nil {
		panic(err)
	}
	if g.code != 0 {
		os.Exit(g.code)
	}
}

func TestToolGlyphsAreDistinctMonochrome(t *testing.T) {
	const n = 48
	bounds := image.Rect(0, 0, n, n)
	fg := color.NRGBA{R: 0xc4, G: 0x3a, B: 0x1a, A: 0xff}
	ids := []ToolID{ToolImage, ToolScreenshot, ToolAndroid, ToolStopwatch, ToolGit, ToolMTP}
	seen := map[string]ToolID{}
	for _, id := range ids {
		img := ebiten.NewImage(n, n)
		drawToolGlyph(img, bounds, id, fg)
		opaque := 0
		var sig []byte
		for y := range n {
			for x := range n {
				c := nrgbaOf(img.At(x, y))
				sig = append(sig, c.A)
				if c.A == 0 {
					if c.R|c.G|c.B != 0 {
						t.Fatalf("%s transparent pixel at %d,%d = %+v", id, x, y, c)
					}
					continue
				}
				opaque++
				if !monoPixel(c, fg) {
					t.Fatalf("%s pixel at %d,%d = %+v, want a shade of %+v", id, x, y, c, fg)
				}
			}
		}
		if opaque == 0 {
			t.Fatalf("%s drew nothing", id)
		}
		key := string(sig)
		if prev, ok := seen[key]; ok {
			t.Fatalf("%s matches %s", id, prev)
		}
		seen[key] = id
		img.Deallocate()
	}

	blank := ebiten.NewImage(n, n)
	drawToolGlyph(blank, bounds, ToolID("missing"), fg)
	if nrgbaOf(blank.At(n/2, n/2)).A != 0 {
		t.Fatal("unknown tool id should not draw")
	}
	blank.Deallocate()
}

func TestBranchMarkMatchesGitGlyph(t *testing.T) {
	const n = 32
	bounds := image.Rect(2, 3, 2+n, 3+n)
	green := color.NRGBA{R: 0x2f, G: 0x9e, B: 0x6b, A: 0xff}
	a := ebiten.NewImage(n+4, n+6)
	b := ebiten.NewImage(n+4, n+6)
	drawBranchGlyph(a, bounds)
	drawBranchMark(b, bounds, green, 1.25)
	for y := range a.Bounds().Dy() {
		for x := range a.Bounds().Dx() {
			ca := nrgbaOf(a.At(x, y))
			cb := nrgbaOf(b.At(x, y))
			if ca != cb {
				t.Fatalf("pixel %d,%d git %+v mark %+v", x, y, ca, cb)
			}
		}
	}
	a.Deallocate()
	b.Deallocate()
}

func TestWriteToolIconPreviews(t *testing.T) {
	dir := os.Getenv("TOOLICON_OUT")
	if dir == "" {
		t.Skip("set TOOLICON_OUT to dump previews")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	fg := color.NRGBA{R: 0x22, G: 0x22, B: 0x22, A: 0xff}
	bg := color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}
	ids := []ToolID{ToolImage, ToolScreenshot, ToolAndroid, ToolStopwatch, ToolGit, ToolMTP}
	for _, size := range []int{24, 64} {
		for _, id := range ids {
			img := ebiten.NewImage(size, size)
			img.Fill(bg)
			drawToolGlyph(img, img.Bounds(), id, fg)
			path := filepath.Join(dir, string(id)+"-"+strconv.Itoa(size)+".png")
			f, err := os.Create(path)
			if err != nil {
				t.Fatal(err)
			}
			err = png.Encode(f, img)
			f.Close()
			img.Deallocate()
			if err != nil {
				t.Fatal(err)
			}
		}
	}
}

func monoPixel(c, fg color.NRGBA) bool {
	if near(c.R, fg.R) && near(c.G, fg.G) && near(c.B, fg.B) {
		return true
	}
	limit := 12
	return channelScale(c.R, fg.R, c.A) <= limit &&
		channelScale(c.G, fg.G, c.A) <= limit &&
		channelScale(c.B, fg.B, c.A) <= limit
}

func channelScale(got, want, alpha uint8) int {
	expect := int(want) * int(alpha) / 255
	d := int(got) - expect
	if d < 0 {
		d = -d
	}
	return d
}

func near(a, b uint8) bool {
	d := int(a) - int(b)
	if d < 0 {
		d = -d
	}
	return d <= 8
}
