package game

import (
	"fmt"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/olivierh59500/kickoff2/internal/engine"
)

var mainItems = []string{"PRACTICE", "SINGLE GAME", "THE LEAGUE", "THE CUP", "INT. FRIENDLY", "ACTION REPLAY", "KIT DESIGN", "OPTIONS", "SPECIAL EVENTS"}

func (g *Game) updateMenu(x, y int, clicked bool) error {
	x -= g.fieldOffset()
	if g.screen == replayScreen {
		if inpututil.IsKeyJustPressed(ebiten.KeyS) {
			g.replaySlow = !g.replaySlow
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyP) {
			g.replayPaused = !g.replayPaused
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyF1) || g.menuActivate {
			g.saveReplay()
			g.menuActivate = false
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyEnter) || inpututil.IsKeyJustPressed(ebiten.KeySpace) || clicked {
			g.replayPlaylist = nil
			g.screen = g.replayReturn
			return nil
		}
		if !g.replayPaused && (!g.replaySlow || g.ticks%2 == 0) {
			g.replayIndex++
		}
		if g.replayIndex >= g.replayLength() {
			if g.advanceReplayPlaylist() {
				return nil
			}
			g.replayIndex = 0
			g.screen = g.replayReturn
		}
		return nil
	}
	if g.screen == helpScreen {
		if clicked || inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
			g.screen = g.helpReturn
			g.CancelInput()
		}
		return nil
	}
	count := len(mainItems)
	if g.updateReplayMenu(x, y, clicked) {
		return nil
	}
	if g.screen == importScreen {
		g.updateImportMenu(x, y, clicked)
		return nil
	}
	if g.updateFeatureMenu(x, y, clicked) {
		return nil
	}
	top, step := 61, 16
	if g.screen == controlsScreen {
		count = 5
		top = 78
		step = 26
	}
	if g.screen == tacticsScreen {
		count = int(engine.FormationCount) + 2
		top = 45
		step = 17
	}
	if g.screen == practiceScreen {
		count = 4
		top = 83
		step = 25
	}
	if g.screen == internationalScreen {
		count = 3
		top = 100
		step = 30
	}
	if g.screen == skillScreen {
		count = 6
		top = 48
		step = 27
	}
	if g.screen == optionsScreen {
		count = 11
		top = 55
		step = 16
	}
	if g.screen == kitScreen {
		count = 5
		top = 103
		step = 23
	}
	if g.screen == competitionScreen {
		return g.updateCompetition(x, y, clicked)
	}
	padX, padY, padActivate := g.menuPadInput()
	navStep := 1
	if g.screen == titleScreen {
		navStep = 3
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowUp) || inpututil.IsKeyJustPressed(ebiten.KeyW) || padY < 0 {
		g.selection = (g.selection + count - navStep) % count
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowDown) || inpututil.IsKeyJustPressed(ebiten.KeyS) || padY > 0 {
		g.selection = (g.selection + navStep) % count
	}
	if g.screen == titleScreen {
		if inpututil.IsKeyJustPressed(ebiten.KeyArrowLeft) || padX < 0 {
			g.selection = (g.selection + 8) % 9
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyArrowRight) || padX > 0 {
			g.selection = (g.selection + 1) % 9
		}
	}

	delta := 1
	activate := padActivate || g.menuActivate || inpututil.IsKeyJustPressed(ebiten.KeyEnter) || inpututil.IsKeyJustPressed(ebiten.KeyZ)
	if g.screen != titleScreen {
		if inpututil.IsKeyJustPressed(ebiten.KeyArrowLeft) || padX < 0 {
			activate = true
			delta = -1
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyArrowRight) || padX > 0 {
			activate = true
		}
	}
	if clicked {
		if g.screen == optionsScreen {
			code := g.originalHit("ko12.men", x, y)
			if code != 0 {
				g.activateOriginalOption(code)
				return nil
			}
		}
		if g.screen == titleScreen {
			for i := range mainItems {
				bx, by := menuBox(i)
				if x >= bx && x < bx+94 && y >= by && y < by+28 {
					g.selection = i
					activate = true
				}
			}
		} else if x >= 20 && x < 300 && y >= top && y < top+count*step {
			g.selection = (y - top) / step
			activate = true
		}
	}
	if !activate {
		return nil
	}
	switch g.screen {
	case titleScreen:
		switch g.selection {
		case 0:
			g.openOriginalPractice()
		case 1:
			g.tournament = nil
			g.practiceMode = -1
			for team, name := range [...]string{"Team A", "Team B"} {
				if len(g.config.UserSquads[team]) == 0 {
					g.config.TeamNames[team] = name
				}
			}
			g.config.SquadFiles = [2]string{}
			g.screen = controlsScreen
			g.selection = 1
		case 2:
			g.beginCompetition(false)
		case 3:
			g.beginCompetition(true)
		case 4:
			g.tournament = nil
			g.practiceMode = -1
			g.config.TeamNames = [2]string{engine.InternationalCountries[0], engine.InternationalCountries[1]}
			g.config.SquadFiles = [2]string{"int1.sqd", "int2.sqd"}
			g.config.UserSquads = [2][]byte{}
			for team := range g.config.SquadBindings {
				g.resetSelectedSquad(team)
				g.resetSquadBindings(team)
			}
			g.screen = internationalScreen
			g.selection = 10
		case 5:
			g.openReplayHome()
		case 6:
			g.screen = kitScreen
			g.kitTeam = -1
			g.selection = 20
		case 7:
			g.screen = optionsScreen
			g.selection = 0
		case 8:
			g.notice = "DATA DISK REQUIRED FOR SPECIAL EVENTS"
		}

	case practiceScreen:
		switch g.selection {
		case 0:
			g.Start(true, false)
		case 1:
			g.Start(false, false)
			g.Match.StartShootout()
		case 2:
			g.Start(true, false)
			g.Match.PracticeRestart(engine.Corner)
		case 3:
			g.Back()
		}
	case internationalScreen:
		if g.selection == 2 {
			g.Start(false, false)
		} else {
			team := g.selection
			index := 0
			for i, n := range countries {
				if n == g.config.TeamNames[team] {
					index = i
				}
			}
			g.config.TeamNames[team] = countries[(index+delta+len(countries))%len(countries)]
		}
	case skillScreen:
		if g.selection == 5 {
			g.screen = optionsScreen
			g.selection = 0
		} else {
			g.config.Skill[g.optionTeam] = 4 - g.selection
			g.saveSettings()
		}
	case controlsScreen:
		switch g.selection {
		case 0:
			g.config.Human[1] = !g.config.Human[1]
		case 1:
			g.changeFixedPlayer(0, delta)
		case 2:
			g.changeFixedPlayer(1, delta)
		case 3:
			g.Start(false, false)
		case 4:
			g.Back()
		}
	case optionsScreen:
		g.changeOption(delta)
		g.saveSettings()
	case kitScreen:
		if g.selection < 4 {
			team := g.selection / 2
			if g.selection%2 == 0 {
				g.kit[team] = (g.kit[team] + 2*delta + 12) % 12
			} else {
				g.kit[team] ^= 1
			}
		} else {
			g.Back()
		}
		g.saveSettings()
	}
	return nil
}

func (g *Game) changeOption(delta int) {
	if delta == 0 {
		delta = 1
	}
	switch g.selection {
	case 0:
		choices := []engine.Pitch{engine.Normal, engine.Wet, engine.Soggy, engine.Artificial, engine.RandomPitch}
		index := 0
		for i, p := range choices {
			if p == g.config.Pitch {
				index = i
			}
		}
		g.config.Pitch = choices[(index+delta+len(choices))%len(choices)]
	case 1:
		choices := []int{180, 300, 600, 1200}
		index := 0
		for i, v := range choices {
			if v == g.config.HalfSeconds {
				index = i
			}
		}
		g.config.HalfSeconds = choices[(index+delta+4)%4]
	case 2:
		g.config.Wind = (g.config.Wind + delta + 4) % 4
	case 3:
		g.config.ExtraTime = !g.config.ExtraTime
	case 4:
		g.config.Aftertouch = !g.config.Aftertouch
	case 5:
		choices := []int{4, 3, 1}
		index := 0
		for i, v := range choices {
			if v == g.config.Skill[0] {
				index = i
			}
		}
		g.config.Skill = [2]int{choices[(index+delta+3)%3], choices[(index+delta+3)%3]}
	case 6:
		choices := []float64{1, 0.5, 0.25}
		index := 0
		for i, v := range choices {
			if v == g.config.Speed {
				index = i
			}
		}
		g.config.Speed = choices[(index+delta+3)%3]
	case 7:
		g.optionTeam = 0
		if delta > 0 {
			g.optionTeam = 1
		}
		g.screen = skillScreen
		g.skillReturn = optionsScreen
		g.selection = 6 - g.config.Skill[g.optionTeam]
	case 8:
		g.optionTeam = 0
		if delta > 0 {
			g.optionTeam = 1
		}
		g.openTactics(g.optionTeam)
	case 9:
		g.config.Referee = (g.config.Referee+1+delta+25)%25 - 1
	case 10:
		g.Back()
	}
}

func yes(v bool) string {
	if v {
		return "YES"
	}
	return "NO"
}
func (g *Game) optionItems() []string {
	mode := "COMPUTER"
	if g.config.Human[1] {
		mode = "PLAYER 2"
	}
	return []string{fmt.Sprintf("DURATION       2 X %d MIN", g.config.HalfSeconds/60), "PITCH          " + g.config.Pitch.String(), fmt.Sprintf("WIND           %d", g.config.Wind), fmt.Sprintf("TEAM A SKILL   %d", g.config.Skill[0]+1), fmt.Sprintf("TEAM B SKILL   %d", g.config.Skill[1]+1), "AFTER TOUCH    " + yes(g.config.Aftertouch), fmt.Sprintf("GAME SPEED     %d PERCENT", int(g.config.Speed*100)), "TEAM B CONTROL " + mode, "TEAM A TACTIC  " + g.config.Formation[0].String(), "TEAM B TACTIC  " + g.config.Formation[1].String(), "EXTRA TIME     " + yes(g.config.ExtraTime), "DONE"}
}

var countries = engine.InternationalCountries[:]

func menuBox(index int) (int, int) {
	row, col := index/3, index%3
	x := []int{0, 17, 32}[row] + col*97
	return x, 119 + row*31
}
