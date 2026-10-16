package game

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

// updateInjuryReserveSelection maps the original F4/F5 selector to desktop
// keys and the two Android reserve buttons. The engine retains the departure,
// field-record swap and return; this UI only submits the ordinary choice.
func (g *Game) updateInjuryReserveSelection(x, y int, clicked bool) {
	team, pending := g.Match.OriginalInjurySubstitutePrompt()
	if !pending || g.paused {
		return
	}
	choice := -1
	if inpututil.IsKeyJustPressed(ebiten.KeyF4) {
		choice = 0
	} else if inpututil.IsKeyJustPressed(ebiten.KeyF5) {
		choice = 1
	}
	if g.touch && clicked && x >= 404 && x < 476 {
		if y >= 146 && y < 180 {
			choice = 0
		} else if y >= 192 && y < 226 {
			choice = 1
		}
	}
	if g.bot != nil {
		// Like the original capture harness, autoplay acknowledges this
		// visible keyboard-only UI separately from joystick policy decisions.
		for slot, used := range g.Match.Teams[team].BenchUsed {
			if !used {
				choice = slot
				break
			}
		}
	}
	if choice >= 0 && g.Match.QueueOriginalInjuryReserve(choice) && g.touch {
		g.CancelInput()
	}
}
