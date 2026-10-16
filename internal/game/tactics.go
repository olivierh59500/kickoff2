package game

import (
	"fmt"

	"github.com/olivierh59500/kickoff2/internal/engine"
)

func (g *Game) openTactics(team int) {
	g.optionTeam, g.tacticPending = team, -1
	g.tacticPendingData, g.tacticPendingName = nil, ""
	g.openFeatureMenu(tacticsScreen, 10)
}

func (g *Game) tacticMenuCodes() []int {
	if g.tacticPending >= 0 {
		return []int{20, 21, 22, 23}
	}
	return []int{10, 11, 12, 13, 14, 15, 16, 17, 18, 19}
}

func (g *Game) activateTactic(code int) {
	if g.tacticPending >= 0 {
		if code < 20 || code > 23 {
			return
		}
		if err := engine.AssignOriginalTactic(&g.config, g.optionTeam, code-20, g.tacticPending, g.tacticPendingData, g.tacticPendingName); err != nil {
			g.notice = err.Error()
			return
		}
		selection := 10 + g.tacticPending
		if g.tacticPending == engine.OriginalCustomTacticID {
			selection = 19
		}
		g.tacticPending, g.tacticPendingData, g.tacticPendingName = -1, nil, ""
		g.selection = selection
		g.saveSettings()
		return
	}
	switch {
	case code >= 10 && code <= 17:
		g.tacticPending = code - 10
		g.selection = 20
	case code == 18:
		g.Back()
	case code == 19:
		g.openImport("tac", g.optionTeam, tacticsScreen)
	}
}

func (g *Game) tacticMenuLabels() map[int]string {
	name := g.config.TeamNames[g.optionTeam]
	if name == "" {
		name = engine.DefaultConfig().TeamNames[g.optionTeam]
	}
	labels := map[int]string{1: "SELECT TACTICS " + name}
	for slot := 0; slot < 4; slot++ {
		labels[20+slot] = engine.OriginalAssignedTacticName(g.config, g.optionTeam, slot)
	}
	for id := 0; id < 8; id++ {
		config := engine.DefaultConfig()
		config.OriginalTactics[0] = engine.OriginalTacticState{Slots: [4]int{id}, Count: 4}
		labels[10+id] = engine.OriginalAssignedTacticName(config, 0, 0)
	}
	return labels
}

func touchPrimaryTeam(config engine.Config) int {
	if !config.Human[0] && config.Human[1] {
		return 1
	}
	return 0
}

func touchTacticSlot(x, y int) (int, bool) {
	if x < 4 || x >= 76 || y < 4 || y >= 64 {
		return 0, false
	}
	column, row := (x-4)/36, (y-4)/30
	if (x-4)%36 >= 34 || (y-4)%30 >= 28 {
		return 0, false
	}
	return row*2 + column, true
}

func tacticTouchLabel(slot int) string { return fmt.Sprintf("T%d", slot+1) }
