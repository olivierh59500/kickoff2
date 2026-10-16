package replayrender

import (
	"strconv"

	"github.com/olivierh59500/kickoff2/internal/engine"
)

// HUD is detached display data; compositing cannot alter a live match or clock.
type HUD struct {
	Scores           [2]int
	RemainingSeconds int
	Message          string
	Practice         bool
	Scanner          engine.OriginalScannerState
	CornerPanel      *engine.OriginalCornerPanelState `json:",omitempty"`
}

// Source19ECA contains seven horizontal and ten vertical endpoint triples.
// 14CF8/14D1A divide each signed endpoint before drawing the exclusive span.
var scannerHorizontal = [][3]int{{0, 0, 912}, {0, 696, 912}, {0, 1392, 912}, {336, 72, 576}, {192, 216, 720}, {192, 1176, 720}, {336, 1320, 576}}
var scannerVertical = [][3]int{{0, 0, 1392}, {912, 0, 1392}, {192, 0, 216}, {336, 0, 72}, {576, 0, 72}, {720, 0, 216}, {720, 1176, 1392}, {192, 1176, 1392}, {336, 1320, 1392}, {576, 1320, 1392}}

func (renderer *Renderer) mapPixel(x, y int, index byte) {
	if x >= 0 && x < 320 && y >= 0 && y < 224 {
		renderer.xorPixel(x, y, index)
	}
}

func (renderer *Renderer) mapPoint(point engine.OriginalScannerPoint, index byte) {
	x, y := int(point.X), int(point.Y)
	if x < 0 || x >= 320 || y < 0 || y >= 224 {
		return
	}
	for row := 0; row < 1+boolInt(point.Big); row++ {
		//14D3C/14DC4 shift C000 in one word. At bit15 the second dot
		// falls out of that word instead of crossing into the next one.
		renderer.xorPixel(x, y+row, index)
		if x&15 != 15 {
			renderer.xorPixel(x+1, y+row, index)
		}
	}
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func (renderer *Renderer) hud(hud HUD) {
	for team, score := range hud.Scores {
		text := strconv.Itoa(score)
		x := 278 - len(text)*6 + team*30
		renderer.text(text, x+16, 224, byte(4-team*2))
	}
	message := hud.Message
	if message == "" && hud.Practice {
		message = "PRACTICE"
	}
	if message != "" {
		renderer.text(message, (26-len([]rune(message))/2)*6+16, 232, 1)
	} else {
		minutes := strconv.Itoa(hud.RemainingSeconds / 60)
		seconds := strconv.Itoa(hud.RemainingSeconds % 60)
		if len(seconds) < 2 {
			seconds = "0" + seconds
		}
		renderer.text(minutes, 123+(5-len(minutes))*6+16, 232, 1)
		renderer.text(":"+seconds, 153+16, 232, 1)
	}
	s := hud.Scanner
	if s.Scale <= 0 {
		return
	}
	if s.Mode > 0 {
		for _, line := range scannerHorizontal {
			for x := line[0] / int(s.Scale); x < line[2]/int(s.Scale); x++ {
				renderer.mapPixel(x+int(s.OriginX), line[1]/int(s.Scale)+int(s.OriginY), 1)
			}
		}
		for _, line := range scannerVertical {
			for y := line[1] / int(s.Scale); y < line[2]/int(s.Scale); y++ {
				renderer.mapPixel(line[0]/int(s.Scale)+int(s.OriginX), y+int(s.OriginY), 1)
			}
		}
		renderer.mapPoint(s.Ball, 3)
	}
	if s.Mode != 0 {
		for team, points := range s.Players {
			for _, point := range points {
				renderer.mapPoint(point, byte(4-team*2))
			}
		}
	}
}
