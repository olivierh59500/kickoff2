package replayrender

import (
	"fmt"

	"github.com/olivierh59500/kickoff2/internal/engine"
	"github.com/olivierh59500/kickoff2/internal/replaydata"
)

// cornerPanel reproduces D662's 96-by-96 XOR blit and EE56's global636 cursor.
// Both use the shared foreground indices and runtime palette, not PNG colors.
func (renderer *Renderer) cornerPanel(panel engine.OriginalCornerPanelState) error {
	if panel.Variant < 0 || panel.Variant >= len(renderer.cornerPanels) || panel.Choice < 0 || panel.Choice > 8 {
		return fmt.Errorf("invalid original corner panel")
	}
	image := &renderer.cornerPanels[panel.Variant]
	for row := 0; row < image.Height; row++ {
		for column := 0; column < image.Width; column++ {
			renderer.xorPixel(int(panel.X)+column, int(panel.Y)+row, image.Pixels[row*image.Width+column])
		}
	}
	offsets := [9][2]int16{{40, 40}, {40, 24}, {56, 24}, {56, 40}, {56, 56}, {40, 56}, {24, 56}, {24, 40}, {24, 24}}
	offset := offsets[panel.Choice]
	return renderer.sprite(replaydata.Command{X: panel.X + offset[0], Y: panel.Y + offset[1], Code: 636})
}
