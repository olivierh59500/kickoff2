// Package replayrender composites original live/movie commands as playfields.
package replayrender

import (
	"fmt"
	"image"
	"image/color"

	"github.com/olivierh59500/kickoff2/assets"
	"github.com/olivierh59500/kickoff2/internal/engine"
	"github.com/olivierh59500/kickoff2/internal/originalgraphics"
	"github.com/olivierh59500/kickoff2/internal/replaydata"
)

// Camera retains the source display offsets, before the visible16-pixel margin.
type Camera struct{ X, Y int32 }

// The common port canvas begins at PAL raster45, while the source match
// DIWSTRT begins at64 and spans224 rows. Menus retain their full256-row canvas.
const ViewTop = 64 - 45

type Renderer struct {
	ground       *originalgraphics.GroundCache
	kits         [2]originalgraphics.Indexed
	keeper, ball originalgraphics.Indexed
	referee      originalgraphics.Indexed
	cornerPanels [2]originalgraphics.Indexed
	font         []byte
	goals        []byte
	palette      [8]color.NRGBA
	scene        originalgraphics.Foreground
	output       *image.RGBA
}

// ForegroundIndices copies the detached XOR layer for independent comparisons
// with original bitplanes, without reading the GPU or changing playback state.
func (renderer *Renderer) ForegroundIndices() []byte { return renderer.scene.Indices() }

// ForegroundPacked preserves the real source layout for memory comparisons.
func (renderer *Renderer) ForegroundPacked() []byte { return renderer.scene.Packed() }

func loadIFF(name string) (originalgraphics.Indexed, error) {
	data, err := assets.Files.ReadFile("original/raw/" + name)
	if err != nil {
		return originalgraphics.Indexed{}, err
	}
	return originalgraphics.DecodeIFF(data)
}

// New prepares the original ground, sprite banks and shared sprite palette.
func New(metadata replaydata.Metadata) (*Renderer, error) {
	if metadata.Pitch > 3 || metadata.Kits[0].Style > 5 || metadata.Kits[1].Style > 5 {
		return nil, fmt.Errorf("invalid original replay graphics setup")
	}
	renderer := &Renderer{output: image.NewRGBA(image.Rect(0, 0, 320, 256))}
	surface := []string{"nor", "wet", "sog", "art"}[metadata.Pitch]
	atlas, err := loadIFF("fbgraph4." + surface)
	if err != nil {
		return nil, err
	}
	tiles, err := assets.Files.ReadFile("original/raw/pdat." + surface)
	if err != nil {
		return nil, fmt.Errorf("missing original pitch map")
	}
	renderer.ground, err = originalgraphics.NewGroundCache(atlas, tiles)
	if err != nil {
		return nil, err
	}
	for team := range renderer.kits {
		renderer.kits[team], err = loadIFF(fmt.Sprintf("style%d%c.iff", metadata.Kits[team].Style+1, 'a'+team))
		if err != nil {
			return nil, err
		}
	}
	if renderer.keeper, err = loadIFF("keeper.iff"); err != nil {
		return nil, err
	}
	if renderer.ball, err = loadIFF("ball.iff"); err != nil {
		return nil, err
	}
	if renderer.referee, err = loadIFF("referee.iff"); err != nil {
		return nil, err
	}
	for variant := range renderer.cornerPanels {
		if renderer.cornerPanels[variant], err = loadIFF(fmt.Sprintf("cornsel%d.iff", variant+1)); err != nil {
			return nil, err
		}
	}
	renderer.font, err = assets.Files.ReadFile("original/raw/fonts.dat")
	if err != nil || len(renderer.font) != 8192 {
		return nil, fmt.Errorf("missing original font data")
	}
	renderer.goals, err = assets.Files.ReadFile("original/raw/goal-sprites.bin")
	if err != nil || len(renderer.goals) != 6*0xa8 {
		return nil, fmt.Errorf("missing original hardware goal sprites")
	}
	copy(renderer.palette[:], renderer.kits[0].Palette)
	wordColor := func(word uint16) color.NRGBA {
		return color.NRGBA{uint8(word>>8&15) * 17, uint8(word>>4&15) * 17, uint8(word&15) * 17, 255}
	}
	renderer.palette[4], renderer.palette[6] = wordColor(metadata.Kits[0].Shirt), wordColor(metadata.Kits[0].Shorts)
	renderer.palette[2], renderer.palette[5] = wordColor(metadata.Kits[1].Shirt), wordColor(metadata.Kits[1].Shorts)
	return renderer, nil
}

// SpriteCell translates the runtime's disjoint global banks to their raw sheets.
// In particular global252 is first-team cell132, not a cell beyond that sheet.
func SpriteCell(global int) (bank string, team, cell int, valid bool) {
	switch {
	case global >= 0 && global < 100:
		return "kit", 0, global, true
	case global < 200 && global >= 100:
		return "kit", 1, global - 100, true
	case global >= 200 && global < 220:
		return "ball", 0, global - 200, true
	case global >= 220 && global < 320:
		return "kit", 0, global - 120, true
	case global >= 320 && global < 420:
		return "kit", 1, global - 220, true
	case global >= 420 && global < 640:
		return "keeper", 0, global - 420, true
	case global >= 640 && global < 680:
		return "kit", 0, global - 440, true
	case global >= 680 && global < 720:
		return "kit", 1, global - 480, true
	case global >= 720 && global < 820:
		return "referee", 0, global - 720, true
	}
	return "", 0, 0, false
}

func (renderer *Renderer) xorPixel(x, y int, value byte) {
	renderer.scene.XORPixel(x, y, value)
}

func (renderer *Renderer) sprite(command replaydata.Command) error {
	x, y := int(command.X), int(command.Y)
	if x < 0 || x >= 335 || y < 0 || y >= 239 {
		return nil
	}
	bank, team, cell, valid := SpriteCell(int(command.Code))
	if !valid {
		return fmt.Errorf("invalid original global sprite %d", command.Code)
	}
	var sheet *originalgraphics.Indexed
	switch bank {
	case "kit":
		sheet = &renderer.kits[team]
	case "ball":
		sheet = &renderer.ball
	case "keeper":
		sheet = &renderer.keeper
	case "referee":
		sheet = &renderer.referee
	}
	blit, visible, err := originalgraphics.NewSpriteBlit(*sheet, cell, x, y)
	if err != nil {
		return err
	}
	if visible {
		renderer.scene.Blit(blit)
	}
	return nil
}

func (renderer *Renderer) caption(command replaydata.Command) {
	text := fmt.Sprintf("%d %s", int(command.X)+1, command.NameText())
	x := 22 // D2D8 adds one word and16 rows; source caption starts at logicalX6.
	renderer.text(text, x, 224, byte(command.Y)&7)
}

func (renderer *Renderer) text(text string, x, y int, index byte) {
	for _, value := range text {
		if value >= 0 && value <= 255 {
			for row := 0; row < 8; row++ {
				bits := renderer.font[(int(value)>>6)*512+(int(value)&63)+row*64]
				for column := 0; column < 8; column++ {
					if bits&(0x80>>uint(column)) != 0 {
						renderer.xorPixel(x+column, y+row, index)
					}
				}
			}
		}
		x += 6
	}
}

// Render draws a detached frame with explicit camera/caption inputs. The pitch
// and XOR sprite plane remain separate until the final dual-playfield composite.
// The returned image belongs to the renderer and is reused at the next call.
func (renderer *Renderer) Render(frame replaydata.Frame, camera Camera, caption *replaydata.Command) (*image.RGBA, error) {
	return renderer.render(frame, camera, caption, nil)
}

// RenderLive adds the source score, clock/message and retained scanner layer.
func (renderer *Renderer) RenderLive(frame replaydata.Frame, camera Camera, caption *replaydata.Command, hud HUD) (*image.RGBA, error) {
	return renderer.render(frame, camera, caption, &hud)
}

func (renderer *Renderer) render(frame replaydata.Frame, camera Camera, caption *replaydata.Command, hud *HUD) (*image.RGBA, error) {
	renderer.scene.Clear()
	for _, command := range frame.Commands {
		switch {
		case command.Code >= 0:
			if err := renderer.sprite(command); err != nil {
				return nil, err
			}
		case command.Code >= -2:
			mask := uint16(0xffff)
			if command.Code == -2 {
				mask = 0xfc3f
			}
			for column := 0; column < 16; column++ {
				if mask&(0x8000>>uint(column)) != 0 {
					renderer.xorPixel(int(command.X)+column, int(command.Y), 1)
				}
			}
		}
	}
	if caption != nil {
		renderer.caption(*caption)
	}
	if hud != nil {
		renderer.hud(*hud)
		if hud.CornerPanel != nil {
			if err := renderer.cornerPanel(*hud.CornerPanel); err != nil {
				return nil, err
			}
		}
	}
	// 10998 writes15-(groundX&15) to the fine-scroll nibble. Its visible
	// fetch therefore starts one pixel after the integer ground coordinate.
	groundX, groundY := int(camera.X>>16)+193, int(camera.Y>>16)+80
	renderer.ground.Prepare(groundX-1, groundY)
	for y := 0; y < 256; y++ {
		for x := 0; x < 320; x++ {
			pixel := color.NRGBA{A: 255}
			if y >= ViewTop && y < ViewTop+224 {
				sx, sy := groundX+x, groundY+y-ViewTop
				if index, present := renderer.ground.Index(sx, sy); present {
					pixel = renderer.ground.Palette[index]
				}
				index := renderer.scene.Index(x+16, y-ViewTop+16)
				if index != 0 {
					pixel = renderer.palette[index]
				}
			}
			offset := y*renderer.output.Stride + x*4
			renderer.output.Pix[offset], renderer.output.Pix[offset+1], renderer.output.Pix[offset+2], renderer.output.Pix[offset+3] = pixel.R, pixel.G, pixel.B, 255
		}
	}
	renderer.goalSprites(camera)
	return renderer.output, nil
}

// Timeline follows the ordinary source movie wait:101 setup callbacks on the
// reset ball, one wait before each target, and100 settle callbacks at target zero.
// Intra-draw extra VBlanks remain a separate timing-validation requirement.
func Timeline(frames []replaydata.Frame) ([]Camera, []*replaydata.Command) {
	camera := Camera{-192 * 65536, -80 * 65536}
	step := func(x, y int32) { camera.X, camera.Y = engine.OriginalCameraStep(camera.X, camera.Y, x, y) }
	for index := 0; index < 101; index++ {
		step(456*65536, 696*65536)
	}
	cameras, captions := make([]Camera, len(frames)), make([]*replaydata.Command, len(frames))
	x, y := int32(456*65536), int32(696*65536)
	var caption *replaydata.Command
	for index, frame := range frames {
		step(x, y)
		x, y = int32(frame.FocusX)*65536, int32(frame.FocusY)*65536
		if index == 0 {
			for delay := 0; delay < 100; delay++ {
				step(x, y)
			}
		}
		for _, command := range frame.Commands {
			if command.Code < -2 {
				copy := command
				caption = &copy
			}
		}
		cameras[index], captions[index] = camera, caption
	}
	return cameras, captions
}
