package game

import (
	"fmt"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"image/color"
)

// updateOriginalSubstitutionInput sends keys to the source main-loop poll.
// Selection does not pause the world: action54 freezes actors while the ball
// and video continue, and SPACE starts the separate departure wait.
func (g *Game) updateOriginalSubstitutionInput(x, y int, clicked bool) {
	if _, injury := g.Match.OriginalInjurySubstitutePrompt(); injury {
		return
	}
	for _, entry := range []struct {
		key  ebiten.Key
		code int
	}{
		{ebiten.KeyF4, 0x3e}, {ebiten.KeyF5, 0x3f}, {ebiten.KeyF9, 0x43}, {ebiten.KeyF10, 0x44},
		{ebiten.KeyArrowUp, 0x48}, {ebiten.KeyArrowDown, 0x50}, {ebiten.KeyArrowLeft, 0x4b}, {ebiten.KeyArrowRight, 0x4d}, {ebiten.KeyF, 0x66},
	} {
		if inpututil.IsKeyJustPressed(entry.key) {
			if g.paused && entry.code >= 0x3e && entry.code <= 0x44 {
				team, reserve := 0, entry.code-0x3e
				if entry.code >= 0x43 {
					team, reserve = 1, entry.code-0x43
				}
				g.Match.RequestOriginalSubstitution(team, reserve)
			} else {
				g.Match.QueueOriginalSubstitutionKey(entry.code)
			}
		}
	}
	_, _, selecting := g.Match.OriginalSubstitutionSelection()
	if !selecting {
		return
	}
	confirm := inpututil.IsKeyJustPressed(ebiten.KeySpace) || inpututil.IsKeyJustPressed(ebiten.KeyEnter)
	if clicked && g.touch && x >= 404 && x < 476 {
		if y >= 146 && y < 180 {
			code := 0x48
			if x >= 440 {
				code = 0x50
			}
			g.Match.QueueOriginalSubstitutionKey(code)
			g.CancelInput()
		} else if y >= 192 && y < 226 {
			confirm = true
		}
	}
	if confirm {
		g.Match.QueueOriginalSubstitutionKey(0x20)
		g.CancelInput()
	}
}

func (g *Game) drawOriginalPlayerView(dst *ebiten.Image) {
	state := g.Match.OriginalSubstitutionDisplay()
	if state.Viewing && state.Cursor >= 0 && state.Cursor < 22 {
		p := g.Match.Teams[state.Cursor/11].Players[state.Cursor%11]
		// The live DIW begins19 rows below the common canvas origin.
		g.text(dst, fmt.Sprintf("%d %s", p.Number, p.Name), 6, 21, originalHUDColors[state.Cursor/11])
	}
	word := int(state.Word)
	if word < 0 {
		word = -word
	}
	if word >= 10 && word < 20 {
		word -= 10
	}
	if word == 1 || word == 2 {
		g.text(dst, fmt.Sprintf("S%d", 10+2*word), 250, 21, color.Black)
	}
}
