package game

import (
	"fmt"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/olivierh59500/kickoff2/internal/engine"
)

// A CPU result is acknowledged before its table/progress becomes current.
// The source calls A15E inside A238, before league/cup bookkeeping returns.
type competitionResult struct {
	next   *engine.Tournament
	names  [2]string
	scores [2]int
	kits   [2]engine.TournamentKit
}

func (g *Game) confirmCompetitionResult() {
	if g.competitionResult == nil {
		return
	}
	g.tournament = g.competitionResult.next
	g.openCompetitionMenu()
}

func (g *Game) updateCompetitionResult(x, y int, clicked bool) {
	dx, _, activate := g.menuPadInput()
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowLeft) {
		dx = -1
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowRight) {
		dx = 1
	}
	g.selection = max(7, min(8, g.selection+dx))
	activate = activate || g.menuActivate || inpututil.IsKeyJustPressed(ebiten.KeyEnter) || inpututil.IsKeyJustPressed(ebiten.KeyZ) || inpututil.IsKeyJustPressed(ebiten.KeyControlLeft)
	if clicked && y >= 190 && y < 215 && ((x >= 33 && x < 144) || (x >= 171 && x < 282)) {
		activate = true
	}
	if activate {
		g.confirmCompetitionResult()
	}
}

func (g *Game) drawCompetitionResult(dst *ebiten.Image) {
	result := g.competitionResult
	labels := map[int]string{1: result.names[0], 2: result.names[1], 3: fmt.Sprint(result.scores[0]), 4: fmt.Sprint(result.scores[1]), 5: "FULL TIME", 6: "A DRAW", 7: "CONTINUE", 8: "CONTINUE", 9: ""}
	if result.scores[0] != result.scores[1] {
		winner := 0
		if result.scores[0] < result.scores[1] {
			winner = 1
		}
		labels[6], labels[9] = "THE WINNER", result.names[winner]
	}
	g.drawOriginalMenu(dst, "ko15.men", func(code int) bool { return code == g.selection }, labels)
	for team, name := range result.names {
		index := team
		for country, candidate := range engine.InternationalCountries {
			if strings.EqualFold(candidate, name) {
				index = country + 2
				break
			}
		}
		design := g.kits[index]
		if result.kits[team].Valid() {
			kit := result.kits[team]
			design = kitDesign{kit.Style, kit.Shirt, kit.Shorts}
		}
		if im := g.coloredKitAsset(fmt.Sprintf("skit%d.bl1", design.Style+1), design, 11, 12); im != nil {
			op := &ebiten.DrawImageOptions{}
			op.GeoM.Translate(5, float64(49+team*19))
			dst.DrawImage(im, op)
		}
	}
}
