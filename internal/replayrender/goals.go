package replayrender

import "encoding/binary"

// goalSprites follows10998->10FBC. Six independent two-plane hardware sprites
// sit above the playfields.11202 clears their registers; copper199F6..19A02
// restores each pair's first sprite color toCCC. Other sprite colors stay black.
// The original bitmap data starts at195D6; the executable itself is not embedded.
func (renderer *Renderer) goalSprites(camera Camera) {
	groundX, groundY := int(camera.X>>16)+192, int(camera.Y>>16)+80
	x, y, height, bottom := 711-groundX, 88-groundY, 25, false
	if groundY > 696 {
		y, height, bottom = 1504-groundY, 40, true
	}
	if x < 0 || x > 448 || y < 0 || y > 320 {
		return
	}
	// DDFSTRT38 begins the fetch at pixel112; the canonical crop skips16
	// fetched pixels. Hardware SPRPOS therefore subtracts128 at this crop.
	x, y = x-128, y-64+ViewTop
	for channel := 0; channel < 6; channel++ {
		for row := 0; row < height; row++ {
			at := channel*0xa8 + 4 + row*4
			first := binary.BigEndian.Uint16(renderer.goals[at:])
			second := binary.BigEndian.Uint16(renderer.goals[at+2:])
			if row == 24 {
				first = 0
				if bottom {
					first = 0xffff
				}
			}
			for column := 0; column < 16; column++ {
				px, py := x+channel*16+column, y+row
				if px >= 0 && px < 320 && py >= ViewTop && py < ViewTop+224 && (first|second)&(0x8000>>uint(column)) != 0 {
					pixel := py*renderer.output.Stride + px*4
					level := byte(0)
					if first&(0x8000>>uint(column)) != 0 && second&(0x8000>>uint(column)) == 0 {
						level = 204
					}
					renderer.output.Pix[pixel], renderer.output.Pix[pixel+1], renderer.output.Pix[pixel+2] = level, level, level
				}
			}
		}
	}
}
