package originalgraphics

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

const ForegroundBytes = 336 * 256 * 3 / 8

// SpriteBlit contains a shifted sprite in the original packed image format.
type SpriteBlit struct {
	Source      [48]uint16
	Destination int
	Shift       uint
}

// NewSpriteBlit packs the source's three 16-pixel planes and original origin.
// Clipping follows the unsigned EE56/EEA0 bounds before drawing starts.
func NewSpriteBlit(sheet Indexed, cell, x, y int) (SpriteBlit, bool, error) {
	if x < 0 || x >= 335 || y < 0 || y >= 239 {
		return SpriteBlit{}, false, nil
	}
	if sheet.Width != 320 || cell < 0 || cell/20*16+16 > sheet.Height || len(sheet.Pixels) != sheet.Width*sheet.Height {
		return SpriteBlit{}, false, fmt.Errorf("invalid original sprite sheet/cell")
	}
	blit := SpriteBlit{Destination: y*126 + x/16*2, Shift: uint(x % 16)}
	sx, sy := cell%20*16, cell/20*16
	for row := 0; row < 16; row++ {
		for plane := 0; plane < 3; plane++ {
			var word uint16
			for column := 0; column < 16; column++ {
				word |= uint16(sheet.Pixels[(sy+row)*sheet.Width+sx+column]>>uint(plane)&1) << uint(15-column)
			}
			blit.Source[row*3+plane] = word
		}
	}
	return blit, true, nil
}

// Foreground stores sprites, text and scanner pixels in one packed image.
type Foreground struct{ packed [ForegroundBytes]byte }

// Clear begins a detached frame without retaining prior renderer-owned pixels.
func (f *Foreground) Clear() { clear(f.packed[:]) }

// Packed copies the source plane layout without exposing mutable live storage.
func (f *Foreground) Packed() []byte { return bytes.Clone(f.packed[:]) }

// XORPixel draws font, controller and scanner masks.
func (f *Foreground) XORPixel(x, y int, index byte) {
	if x < 0 || x >= 336 || y < 0 || y >= 256 {
		return
	}
	base := y*126 + x/8
	mask := byte(0x80) >> uint(x%8)
	for plane := 0; plane < 3; plane++ {
		if index&(1<<uint(plane)) != 0 {
			f.packed[base+plane*42] ^= mask
		}
	}
}

// Index reads a validated foreground position as one shared-palette index.
func (f *Foreground) Index(x, y int) byte {
	base := y*126 + x/8
	mask := byte(0x80) >> uint(x%8)
	var index byte
	for plane := 0; plane < 3; plane++ {
		if f.packed[base+plane*42]&mask != 0 {
			index |= 1 << uint(plane)
		}
	}
	return index
}

// Indices supplies a detached inspection image of the three source planes.
func (f *Foreground) Indices() []byte {
	result := make([]byte, 336*256)
	for y := 0; y < 256; y++ {
		for x := 0; x < 336; x++ {
			result[y*336+x] = f.Index(x, y)
		}
	}
	return result
}

// Blit draws both words of each sprite row directly. The right-hand word
// preserves the original visible spill across interleaved plane boundaries.
func (f *Foreground) Blit(blit SpriteBlit) {
	for row, source := range blit.Source {
		address := blit.Destination + row*42
		left := source >> blit.Shift
		right := uint16(uint32(source) << (16 - blit.Shift))
		binary.BigEndian.PutUint16(f.packed[address:], binary.BigEndian.Uint16(f.packed[address:])^left)
		binary.BigEndian.PutUint16(f.packed[address+2:], binary.BigEndian.Uint16(f.packed[address+2:])^right)
	}
}
