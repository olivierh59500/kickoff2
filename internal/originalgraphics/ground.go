package originalgraphics

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image/color"
)

const (
	GroundColumns         = 21
	GroundRows            = 15
	GroundLineBytes       = 126
	GroundTileRowBytes    = 16 * GroundLineBytes
	GroundRingBytes       = GroundRows * GroundTileRowBytes
	GroundAllocationBytes = 0x7e00
)

// GroundBlit is one source1137E/113FA tile copy. A tile occupies 48 one-word
// blitter rows: 16 screen rows interleaved across the three ground planes.
type GroundBlit struct {
	MapOffset   int
	Destination uint32
}

// GroundRowBlits follows the source row loop's contiguous map and word steps.
func GroundRowBlits(mapOffset int, destination uint32) [GroundColumns]GroundBlit {
	var result [GroundColumns]GroundBlit
	for index := range result {
		result[index] = GroundBlit{mapOffset + index, destination + uint32(index*2)}
	}
	return result
}

// GroundColumnBlits follows the source column loop, including its ring wrap.
func GroundColumnBlits(mapOffset, mapWidth int, destination, origin uint32) [GroundRows]GroundBlit {
	var result [GroundRows]GroundBlit
	for index := range result {
		result[index] = GroundBlit{mapOffset, destination}
		mapOffset += mapWidth
		destination += GroundTileRowBytes
		if destination >= origin+GroundRingBytes {
			destination -= GroundRingBytes
		}
	}
	return result
}

// GroundWork reports actual cache copies. It supplies work quantities rather
// than an observed interrupt schedule or an estimated source CPU duration.
type GroundWork struct{ InitialTiles, RowTiles, ColumnTiles int }

// GroundCache keeps the source's 21-by-15 tile window in interleaved bitplanes.
// Horizontal scrolling shifts the shared plane origin by one word. Vertical
// scrolling recycles one of fifteen 16-pixel rows in the 240-line ring.
type GroundCache struct {
	Width, Height         int
	Palette               []color.NRGBA
	atlas                 Indexed
	mapWidth, mapHeight   int
	tileMap               []byte
	packed                []byte
	tileX, tileY, rowSlot int
	ready                 bool
	work                  GroundWork
}

// NewGroundCache validates the recovered atlas/map without expanding the pitch.
func NewGroundCache(atlas Indexed, mapData []byte) (*GroundCache, error) {
	if len(mapData) < 4 {
		return nil, fmt.Errorf("missing original ground map")
	}
	width, height := int(binary.BigEndian.Uint16(mapData)), int(binary.BigEndian.Uint16(mapData[2:]))
	if width != 80 || height != 96 || len(mapData) != 4+width*height || atlas.Width != 320 || atlas.Height%16 != 0 || len(atlas.Pixels) != atlas.Width*atlas.Height || len(atlas.Palette) < 8 {
		return nil, fmt.Errorf("invalid original ground geometry")
	}
	for _, id := range mapData[4:] {
		if id == 0 || int(id) > atlas.Width/16*(atlas.Height/16) {
			return nil, fmt.Errorf("invalid original ground tile")
		}
	}
	return &GroundCache{Width: width * 16, Height: height * 16, Palette: atlas.Palette, atlas: atlas, mapWidth: width, mapHeight: height,
		tileMap: bytes.Clone(mapData[4:]), packed: make([]byte, GroundAllocationBytes)}, nil
}

// SeedPacked initializes one genuine source snapshot for isolated continuation
// checks. Later source buffers are outputs to compare, never cache inputs.
func (g *GroundCache) SeedPacked(scrollX, scrollY, rowSlot int, packed []byte) error {
	if len(packed) != GroundAllocationBytes || scrollX < 0 || scrollX >= g.Width-320 || scrollY < 0 || scrollY >= g.Height-224 || rowSlot < 0 || rowSlot >= GroundRows {
		return fmt.Errorf("invalid original ground cache snapshot")
	}
	copy(g.packed, packed)
	g.tileX, g.tileY, g.rowSlot, g.ready = scrollX/16, scrollY/16, rowSlot, true
	g.work = GroundWork{}
	return nil
}

// Packed copies the source-layout cache for independent memory comparisons.
func (g *GroundCache) Packed() []byte { return bytes.Clone(g.packed) }

// Work describes the most recent Prepare call, independently from host speed.
func (g *GroundCache) Work() GroundWork { return g.work }

// Prepare performs source-order column then row refreshes. Detached first views
// and replay seeks initialize a complete window instead of relying on old data.
func (g *GroundCache) Prepare(scrollX, scrollY int) {
	x, y := max(0, min(g.mapWidth-GroundColumns, scrollX/16)), max(0, min(g.mapHeight-GroundRows, scrollY/16))
	g.work = GroundWork{}
	if !g.ready || x-g.tileX > 1 || x-g.tileX < -1 || y-g.tileY > 1 || y-g.tileY < -1 {
		g.tileX, g.tileY, g.rowSlot, g.ready = x, y, 0, true
		clear(g.packed)
		for row := 0; row < GroundRows; row++ {
			g.copyRow((y+row)*g.mapWidth+x, uint32(x*2+row*GroundTileRowBytes))
		}
		g.work = GroundWork{InitialTiles: GroundColumns * GroundRows}
		return
	}
	origin := uint32(g.tileX * 2)
	pointer := origin + uint32(g.rowSlot*GroundTileRowBytes)
	if x > g.tileX {
		g.copyColumn(g.tileY*g.mapWidth+x+20, pointer+42, origin)
		g.work.ColumnTiles = GroundRows
	} else if x < g.tileX {
		origin -= 2
		pointer -= 2
		g.copyColumn(g.tileY*g.mapWidth+x, pointer, origin)
		g.work.ColumnTiles = GroundRows
	}
	g.tileX = x
	origin = uint32(x * 2)
	if y > g.tileY {
		g.copyRow((y+14)*g.mapWidth+x, origin+uint32(g.rowSlot*GroundTileRowBytes))
		g.rowSlot = (g.rowSlot + 1) % GroundRows
		g.work.RowTiles = GroundColumns
	} else if y < g.tileY {
		g.rowSlot = (g.rowSlot + GroundRows - 1) % GroundRows
		g.copyRow(y*g.mapWidth+x, origin+uint32(g.rowSlot*GroundTileRowBytes))
		g.work.RowTiles = GroundColumns
	}
	g.tileY = y
}

func (g *GroundCache) copyRow(offset int, destination uint32) {
	for _, blit := range GroundRowBlits(offset, destination) {
		g.copyTile(blit)
	}
}

func (g *GroundCache) copyColumn(offset int, destination, origin uint32) {
	for _, blit := range GroundColumnBlits(offset, g.mapWidth, destination, origin) {
		g.copyTile(blit)
	}
}

func (g *GroundCache) copyTile(blit GroundBlit) {
	id := int(g.tileMap[blit.MapOffset]) - 1
	sx, sy := (id%20)*16, (id/20)*16
	for row := 0; row < 16; row++ {
		for plane := 0; plane < 3; plane++ {
			var word uint16
			for column := 0; column < 16; column++ {
				word |= uint16(g.atlas.Pixels[(sy+row)*g.atlas.Width+sx+column]>>uint(plane)&1) << uint(15-column)
			}
			destination := int(blit.Destination) + row*GroundLineBytes + plane*42
			binary.BigEndian.PutUint16(g.packed[destination:], word)
		}
	}
}

// Index reads the current window through its fine-scroll and circular-row
// offsets. Out-of-map detached views retain the compositor's black border.
func (g *GroundCache) Index(worldX, worldY int) (byte, bool) {
	if !g.ready || worldX < 0 || worldX >= g.Width || worldY < 0 || worldY >= g.Height {
		return 0, false
	}
	x, y := worldX-g.tileX*16, worldY-g.tileY*16
	if x < 0 || x >= GroundColumns*16 || y < 0 || y >= GroundRows*16 {
		return 0, false
	}
	row := (g.rowSlot*16 + y) % (GroundRows * 16)
	base := g.tileX*2 + row*GroundLineBytes + x/16*2
	mask := uint16(0x8000) >> uint(x%16)
	var index byte
	for plane := 0; plane < 3; plane++ {
		if binary.BigEndian.Uint16(g.packed[base+plane*42:])&mask != 0 {
			index |= 1 << uint(plane)
		}
	}
	return index, true
}
