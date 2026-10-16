package game

import "github.com/olivierh59500/kickoff2/internal/engine"

// selectedSquadRecordSlot resolves a player by its source-file record. Player
// seeds are display data and are neither unique nor guaranteed to be nonzero.
func (g *Game) selectedSquadRecordSlot(team, recordIndex int) int {
	return engine.SquadSelectionSlot(g.config.SelectedSquadRecords[team], recordIndex)
}

// resetSelectedSquad removes both the canonical record references and the
// legacy seed mirror. The next squad-screen entry derives the source shirts.
func (g *Game) resetSelectedSquad(team int) {
	g.config.SelectedSquadRecords[team] = [13]uint8{}
	g.config.HasSelectedSquadRecords[team] = false
	g.config.SelectedSquadIDs[team] = [13]uint32{}
}

// syncSelectedSquadIDs keeps old settings/readers useful without using player
// seeds as identity inside the menu.
func (g *Game) syncSelectedSquadIDs(pool []engine.SquadPlayer) {
	team := g.squadTeam
	g.config.SelectedSquadIDs[team] = engine.SquadSelectionIDs(pool, g.config.SelectedSquadRecords[team])
}

func (g *Game) initializeUnassignedSquadControllers(pool []engine.SquadPlayer) {
	team := g.squadTeam
	if !g.config.HasUnassignedSquadControllers[team] {
		g.config.UnassignedSquadControllers[team] = [64]uint8{}
		for _, player := range pool {
			if g.selectedSquadRecordSlot(team, player.RecordIndex) < 0 {
				g.config.UnassignedSquadControllers[team][player.RecordIndex] = player.Record[21] >> 4
			}
		}
		g.config.HasUnassignedSquadControllers[team] = true
	}
	allowed := originalSquadModePorts(g.originalSquadMode())
	for index, controller := range g.config.UnassignedSquadControllers[team] {
		if index >= len(pool) || controller != allowed[0] && controller != allowed[1] {
			g.config.UnassignedSquadControllers[team][index] = 0
		}
	}
	for _, player := range pool {
		if g.selectedSquadRecordSlot(team, player.RecordIndex) >= 0 {
			g.config.UnassignedSquadControllers[team][player.RecordIndex] = 0
		}
	}
}

func (g *Game) squadPoolController(player engine.SquadPlayer) uint8 {
	team := g.squadTeam
	if slot := g.selectedSquadRecordSlot(team, player.RecordIndex); slot >= 0 {
		role := g.config.SquadBindings[team][slot]
		if role == 0 || role > 3 {
			return 0
		}
		ports := g.config.SquadBindingPorts[team]
		if ports[0] == 0 {
			ports = g.currentSquadBindingPorts(team)
		}
		return ports[role-1]
	}
	return g.config.UnassignedSquadControllers[team][player.RecordIndex]
}
