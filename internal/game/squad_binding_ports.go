package game

import "github.com/olivierh59500/kickoff2/internal/engine"

// configuredSquadBindingPorts describes stored roles independently of which
// squad menu is currently open. Two-player Practice uses J2 as the home partner.
func configuredSquadBindingPorts(config engine.Config, team int) [3]uint8 {
	primary := uint8(team + 1)
	if team == 1 && !config.Human[0] && config.Human[1] {
		primary = 1
	}
	partner := uint8(team + 3)
	if team == 0 && config.Cooperative[0] && !config.Human[1] {
		partner = 2
	}
	return [3]uint8{primary, partner, 3 - primary}
}

func (g *Game) rememberSquadBindingPorts() {
	for team := range g.config.SquadBindingPorts {
		if g.config.SquadBindingPorts[team][0] == 0 && (g.config.HasSquadBindings[team] || g.config.FixedPlayer[team] > 0 || g.config.PartnerPlayer[team] > 0) {
			g.config.SquadBindingPorts[team] = configuredSquadBindingPorts(g.config, team)
		}
	}
}

func (g *Game) currentSquadBindingPorts(team int) [3]uint8 {
	ports := configuredSquadBindingPorts(g.config, team)
	ports[1] = uint8(team + 3)
	if team == 0 && g.practiceMode >= 0 && g.practicePlayers == 2 {
		ports[1] = 2
	}
	return ports
}

func originalSquadModePorts(mode int) [2]uint8 {
	switch mode {
	case 0:
		return [2]uint8{1, 0}
	case 1:
		return [2]uint8{1, 2}
	case 2:
		return [2]uint8{2, 0}
	case 3:
		return [2]uint8{1, 3}
	case 4:
		return [2]uint8{2, 4}
	}
	return [2]uint8{}
}

func (g *Game) squadRoleForController(controller uint8) uint8 {
	allowed := originalSquadModePorts(g.originalSquadMode())
	if controller == 0 || controller != allowed[0] && controller != allowed[1] {
		return 0
	}
	ports := g.currentSquadBindingPorts(g.squadTeam)
	if controller == ports[0] {
		return 1
	}
	if g.config.Cooperative[g.squadTeam] && controller == ports[1] {
		return 2
	}
	if controller == ports[2] {
		return 3
	}
	return 0
}

// normalizeSquadBindingPorts follows 7246..727A: keep only the current menu's
// permitted physical numbers. Reclassifying a retained number changes its role,
// never the joystick to which the original selection was assigned.
func (g *Game) normalizeSquadBindingPorts() {
	team := g.squadTeam
	old := g.config.SquadBindingPorts[team]
	if old[0] == 0 {
		old = g.currentSquadBindingPorts(team)
	}
	for slot, role := range g.config.SquadBindings[team] {
		g.config.SquadBindings[team][slot] = 0
		if role > 0 && role <= 3 {
			g.config.SquadBindings[team][slot] = g.squadRoleForController(old[role-1])
		}
	}
	g.config.SquadBindingPorts[team] = g.currentSquadBindingPorts(team)
	g.syncSquadBindingSlots()
}
