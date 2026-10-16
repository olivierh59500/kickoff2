package game

import (
	"fmt"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

func (g *Game) competitionMenuSource() string {
	if g.tournament != nil && g.tournament.Cup {
		return "ko18.men"
	}
	return "ko14.men"
}

func (g *Game) competitionRows() [8]int {
	var rows [8]int
	if g.tournament == nil {
		return rows
	}
	for rank, standing := range g.tournament.Table {
		rows[rank] = standing.Team
	}
	if g.tournament.Cup {
		for team, rank := range g.tournament.DrawPositions {
			rows[rank] = team
		}
	}
	return rows
}

func (g *Game) toggleCompetitionCountry(row int) {
	if g.tournament == nil || row < 0 || row >= 8 {
		return
	}
	team := g.competitionRows()[row]
	g.tournament.HumanTeams[team] = !g.tournament.HumanTeams[team]
	g.competitionHumans = g.tournament.HumanTeams
}

func (g *Game) activateCompetition(code int) {
	if code >= 10 && code <= 66 && (code-10)%8 == 0 {
		g.competitionRowAction(ebiten.KeyEnter)
		return
	}
	switch code {
	case 1:
		g.continueTournament()
	case 2:
		g.saveTournament()
		g.exportCompetitionDocument()
	case 3:
		g.openCompetitionLoad()
	case 4:
		g.Back()
	}
}

func (g *Game) updateCompetition(x, y int, clicked bool) error {
	if g.tournament == nil {
		return nil
	}
	if g.competitionResult != nil {
		g.updateCompetitionResult(x, y, clicked)
		return nil
	}
	if g.competitionName.active {
		g.updateCompetitionName(clicked)
		return nil
	}
	dx, dy, fire := g.menuPadInput()
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowLeft) {
		dx = -1
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowRight) {
		dx = 1
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowUp) {
		dy = -1
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowDown) {
		dy = 1
	}
	if g.selection < 10 {
		if dy < 0 && dx == 0 {
			g.selection = 66
		} else if dx != 0 && dy == 0 {
			g.selection = max(1, min(4, g.selection+dx))
		}
	} else if dx == 0 {
		if dy < 0 {
			g.selection = max(10, g.selection-8)
		} else if dy > 0 {
			g.selection += 8
			if g.selection > 66 {
				g.selection = 1
			}
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyF1) && g.selection >= 10 {
		g.toggleCompetitionCountry((g.selection - 10) / 8)
	}
	for _, key := range []ebiten.Key{ebiten.KeyF3, ebiten.KeyF5} {
		if inpututil.IsKeyJustPressed(key) {
			g.competitionRowAction(key)
			return nil
		}
	}
	if clicked && g.touch {
		if key, ok := competitionEditTouchKey(x, y); ok {
			g.competitionRowAction(key)
			return nil
		}
	}
	if clicked {
		for _, record := range g.menus[g.competitionMenuSource()].Records {
			if record.Kind != "button" || x < record.X || x >= record.X+record.Width || y < record.Y+24 || y >= record.Y+24+record.Height {
				continue
			}
			code := record.Selection
			if code >= 100 && code <= 107 {
				g.selection = 10 + (code-100)*8
				g.toggleCompetitionCountry(code - 100)
				return nil
			}
			if code >= 10 && code <= 66 && (code-10)%8 == 0 {
				g.selection = code
				g.activateCompetition(code)
				return nil
			}
			if code >= 1 && code <= 4 {
				g.selection = code
				g.activateCompetition(code)
				return nil
			}
		}
	}
	if fire || g.menuActivate || inpututil.IsKeyJustPressed(ebiten.KeyEnter) || inpututil.IsKeyJustPressed(ebiten.KeyNumpadEnter) || inpututil.IsKeyJustPressed(ebiten.KeyZ) || inpututil.IsKeyJustPressed(ebiten.KeyControlLeft) {
		g.activateCompetition(g.selection)
	}
	return nil
}

func (g *Game) competitionLabels() map[int]string {
	labels := map[int]string{}
	rows := g.competitionRows()
	controls := g.tournament.CountryControls()
	for row, team := range rows {
		base := 10 + row*8
		labels[base] = g.tournament.Names[team]
		labels[100+row] = [...]string{"C", "J", "J 1", "J 2"}[controls[team]]
		for column := 1; column <= 7; column++ {
			labels[base+column] = ""
		}
		if !g.tournament.Cup {
			standing := g.tournament.Table[row]
			for column, value := range []int{standing.Played, standing.Won, standing.Drawn, standing.Lost, standing.For, standing.Against, standing.Points} {
				labels[base+column+1] = fmt.Sprint(value)
			}
		}
	}
	if g.tournament.Cup {
		var counts [4]int
		for _, fixture := range g.tournament.Fixtures {
			letter := string(rune('A' + counts[fixture.Round]))
			counts[fixture.Round]++
			for _, team := range []int{fixture.Home, fixture.Away} {
				labels[10+g.tournament.DrawPositions[team]*8+fixture.Round] = letter
			}
			if fixture.Played {
				labels[10+g.tournament.DrawPositions[fixture.Winner]*8+fixture.Round+1] = "*"
			}
		}
	}
	return labels
}

func (g *Game) drawCompetition(dst *ebiten.Image) {
	if g.tournament == nil {
		return
	}
	if g.competitionResult != nil {
		g.drawCompetitionResult(dst)
		return
	}
	menu := g.competitionMenuSource()
	g.drawOriginalMenu(dst, menu, func(code int) bool { return code == g.selection && !g.competitionName.active }, g.competitionLabels())
	if fixture, ok := g.tournament.Next(); ok {
		for row, team := range g.competitionRows() {
			if team != fixture.Home && team != fixture.Away {
				continue
			}
			for _, record := range g.menus[menu].Records {
				if record.Selection != 100+row {
					continue
				}
				color := amigaMenuColor(g.menus[menu].Palette[7^15])
				x, y, w, h := float64(record.X), float64(record.Y+24), float64(record.Width), float64(record.Height)
				rect(dst, x, y, w, 1, color)
				rect(dst, x, y+h-1, w, 1, color)
				rect(dst, x, y, 1, h, color)
				rect(dst, x+w-1, y, 1, h, color)
			}
		}
	}
	if g.notice != "" {
		g.centered(dst, g.notice, 241, ink)
	}
	if g.competitionName.active {
		g.drawCompetitionNameCursor(dst)
	} else if g.touch && g.selection >= 10 {
		for index, label := range []string{"F3 LOAD TEAM", "F5 TACTICS"} {
			rect(dst, float64(10+index*160), 225, 140, 14, panel)
			color := gold
			if index == 0 && g.competitionSquadImportLocked {
				color = ink
			}
			g.text(dst, label, 35+index*160, 228, color)
		}
	}
}
