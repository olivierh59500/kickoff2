package game

import (
	"fmt"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/olivierh59500/kickoff2/internal/engine"
)

// updatePeriodMenu presents A104/A1DC/A15E after the source exit latch. Menu
// acknowledgments perform setup only and never consume a world update.
func (g *Game) updatePeriodMenu(x, y int, clicked bool) {
	if !g.periodMenuActive {
		g.periodMenuActive, g.periodMenuChoice = true, 7
		g.periodMenuReleased, g.periodMenuFire, g.periodMenuDirection = false, false, -1
	}
	if g.touch {
		g.readTouchControls()
	}
	primary := 0
	if !g.Match.Config.Human[0] && g.Match.Config.Human[1] {
		primary = 1
	}
	input := g.readControllerInput()[primary]
	direction := -1
	if input.Move.Len() > .4 {
		direction = engine.OriginalDirectionIndex(input.Move)
	}
	if g.periodMenuReleased && direction != g.periodMenuDirection {
		if direction == 6 {
			g.periodMenuChoice = 7
		} else if direction == 2 {
			g.periodMenuChoice = 8
		}
	}
	g.periodMenuDirection = direction
	if !input.Fire {
		g.periodMenuReleased = true
	}
	activate := g.periodMenuReleased && input.Fire && !g.periodMenuFire || inpututil.IsKeyJustPressed(ebiten.KeyEnter)
	g.periodMenuFire = input.Fire
	x -= g.fieldOffset()
	if clicked && y >= 190 && y < 215 {
		if x >= 33 && x < 144 {
			g.periodMenuChoice, activate = 7, true
		} else if x >= 171 && x < 282 {
			g.periodMenuChoice, activate = 8, true
		}
	}
	if !activate {
		return
	}
	if g.Match.Phase == engine.HalfTime && (g.periodMenuChoice == 7 || g.Match.Config.Competition) {
		g.Match.ContinueOriginalPeriod()
		if g.audio != nil {
			g.matchAudio(engine.HalfTime)
		}
		g.periodMenuActive = false
		g.CancelInput()
		g.updateCamera()
		return
	}
	if g.Match.Phase == engine.FullTime && g.Match.Config.Competition && g.tournament != nil {
		g.finishTournamentMatch()
	} else {
		g.screen, g.paused, g.periodMenuActive = titleScreen, false, false
		g.CancelInput()
	}
}

func (g *Game) periodMenuLabels() map[int]string {
	m := g.Match
	labels := map[int]string{1: m.Teams[0].Name, 2: m.Teams[1].Name,
		3: fmt.Sprint(m.Teams[0].Score), 4: fmt.Sprint(m.Teams[1].Score),
		5: "", 6: "HALF TIME", 7: "CONTINUE", 8: "QUIT", 9: ""}
	if m.Config.Competition {
		labels[8] = "CONTINUE"
	}
	period := m.OriginalPeriod()
	if period.HalfFlag != 0 && !period.ExtraStage && m.Phase == engine.HalfTime {
		labels[6] = "EXTRA TIME"
	}
	if m.Phase == engine.FullTime {
		labels[5], labels[6], labels[8] = "FULL TIME", "A DRAW", "CONTINUE"
		if m.Winner >= 0 {
			labels[6], labels[9] = "THE WINNER", m.Teams[m.Winner].Name
		}
	}
	return labels
}

func (g *Game) drawPeriodMenu(dst *ebiten.Image) {
	choice := g.periodMenuChoice
	if !g.periodMenuActive {
		choice = 7
	}
	g.drawOriginalMenu(dst, "ko15.men", func(code int) bool { return code == choice }, g.periodMenuLabels())
	for team := 0; team < 2; team++ {
		design := g.teamKitDesign(team)
		if im := g.coloredKitAsset(fmt.Sprintf("skit%d.bl1", design.Style+1), design, 11, 12); im != nil {
			op := &ebiten.DrawImageOptions{}
			op.GeoM.Translate(5, float64(49+team*19))
			dst.DrawImage(im, op)
		}
	}
}
