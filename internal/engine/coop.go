package engine

import "math"

// UpdateControllers samples the original four-controller arrangement: slots0/1
// control opposing teams A/B; slots2/3 join A/B. Partners control fixed outfield
// shirts. Update remains compatible with existing two-controller hosts.
func (m *Match) UpdateControllers(input [4]Input) {
	modalHistorySampled := m.originalCornerSelecting || m.originalSubstitutionBlocked || m.originalPALHistorySampled
	if modalHistorySampled && !m.originalPALHistorySampled {
		// Age all old samples before replacing either parallel-port input.
		m.updateOriginalInputHistory()
	}
	m.cooperativeFrame = true
	for team := range m.cooperativeInputs {
		m.cooperativeInputs[team] = input[team+2]
		if m.cooperativeInputs[team].Move.Len() > 1 {
			m.cooperativeInputs[team].Move = m.cooperativeInputs[team].Move.Unit()
		}
		if slot := m.cooperativeInputs[team].TacticSlot; m.Config.Human[team] && m.Config.Cooperative[team] && slot != nil {
			m.QueueOriginalTacticSlot(team, *slot)
		}
	}
	m.update([2]Input{input[0], input[1]}, modalHistorySampled)
	m.cooperativeFrame = false
}

// PartnerPlayer returns an eligible fixed co-op player, or -1 for no partner.
// Goalkeepers retain dedicated keeper behavior. Removed partners remain bound
// but do not receive normal input while inactive.
func (m *Match) PartnerPlayer(team int) int {
	if team < 0 || team >= 2 || !m.Config.Human[team] {
		return -1
	}
	if m.Config.HasSquadBindings[team] {
		partner := -1
		for index := 0; index < 11; index++ {
			role := m.Config.SquadBindings[team][index]
			p := &m.Teams[team].Players[index]
			if (role == 2 || role == 3) && p.originalState != 0 && !p.SentOff {
				// 14AE4 overwrites the retained partner pointer for every
				// active non-primary binding visited by the forward actor scan.
				partner = index
			}
		}
		return partner
	}
	index := m.boundPartnerPlayer(team)
	if index < 0 || m.Teams[team].Players[index].SentOff {
		return -1
	}
	return index
}

// boundPartnerPlayer retains the assignment independently of actor activity.
// Original 13188 searches this binding when staging a close free kick.
func (m *Match) boundPartnerPlayer(team int) int {
	if team < 0 || team >= 2 || !m.Config.Human[team] {
		return -1
	}
	if m.Config.HasSquadBindings[team] {
		for index := 0; index < 11; index++ {
			role := m.Config.SquadBindings[team][index]
			if role == 2 || role == 3 {
				return index
			}
		}
		return -1
	}
	if !m.Config.Cooperative[team] {
		return -1
	}
	index := m.Config.PartnerPlayer[team]
	if index < 1 || index >= 11 {
		return -1
	}
	return index
}

func (m *Match) boundPrimaryPlayer(team int) int {
	if team < 0 || team >= 2 || !m.Config.Human[team] {
		return -1
	}
	if m.Config.HasSquadBindings[team] {
		for slot := 0; slot < 11; slot++ {
			if m.Config.SquadBindings[team][slot] == 1 {
				return slot
			}
		}
		return -1
	}
	if slot := m.Config.FixedPlayer[team]; slot > 0 && slot < 11 {
		return slot
	}
	return -1
}

// initialBoundPrimaryPlayer mirrors 1484E's forward scan and repeated pointer
// overwrite. Reading the explicit roster backwards returns the same last match.
// The forward boundPrimaryPlayer remains separate for 13130 restart staging.
func (m *Match) initialBoundPrimaryPlayer(team int) int {
	if team < 0 || team >= 2 || !m.Config.Human[team] {
		return -1
	}
	if m.Config.HasSquadBindings[team] {
		for slot := 10; slot >= 0; slot-- {
			if m.Config.SquadBindings[team][slot] == 1 {
				return slot
			}
		}
		return -1
	}
	return m.boundPrimaryPlayer(team)
}

// explicitPlayerController resolves the B4 pointer stored on one particular
// playing record. Duplicate bindings therefore keep their individual routes.
func (m *Match) explicitPlayerController(team, player int) (int, bool) {
	if team < 0 || team >= 2 || player < 0 || player >= 13 || !m.Config.HasSquadBindings[team] {
		return 0, false
	}
	switch m.Config.SquadBindings[team][player] {
	case 1:
		return team, true
	case 2:
		return team + 2, true
	case 3:
		return 1 - team, true
	default:
		return 0, false
	}
}

func (m *Match) secondaryBoundPlayer(team, player int) bool {
	if team < 0 || team >= 2 || player < 0 || player >= 11 {
		return false
	}
	if !m.Config.HasSquadBindings[team] {
		return player == m.PartnerPlayer(team)
	}
	role := m.Config.SquadBindings[team][player]
	p := &m.Teams[team].Players[player]
	return (role == 2 || role == 3) && p.originalState != 0 && !p.SentOff
}

func (m *Match) playerInput(team, player int) (current, previous Input, controlled bool) {
	if !m.Config.Human[team] {
		return Input{}, Input{}, false
	}
	if controller, explicit := m.explicitPlayerController(team, player); explicit {
		current, previous = m.controllerInputs(controller)
		return current, previous, true
	}
	if player >= 0 && player == m.PartnerPlayer(team) {
		current, previous = m.controllerInputs(team + 2)
		return current, previous, true
	}
	return m.controls[team], m.previous[team], m.Teams[team].primaryControls(player)
}

// originalPlayerController identifies the existing native input slot. A foreign
// B4 binding can share the opposing primary input without adding another pad.
func (m *Match) originalPlayerController(team, player int) int {
	if controller, explicit := m.explicitPlayerController(team, player); explicit {
		return controller
	}
	if player >= 0 && player == m.PartnerPlayer(team) {
		return team + 2
	}
	return team
}

func (m *Match) controllerInputs(controller int) (Input, Input) {
	if controller < 2 {
		return m.controls[controller], m.previous[controller]
	}
	return m.cooperativeInputs[controller-2], m.cooperativePrevious[controller-2]
}

func (m *Match) nearestPrimaryPlayer(team, exclude int) int {
	best, distance := -1, math.MaxFloat64
	target := m.Ball.Pos.Add(m.Ball.Vel.Mul(0.08))
	for i := 1; i < 11; i++ {
		p := &m.Teams[team].Players[i]
		if m.secondaryBoundPlayer(team, i) || i == exclude || p.SentOff {
			continue
		}
		if d := p.Pos.Sub(target).Len(); d < distance {
			best, distance = i, d
		}
	}
	return best
}

// updateControllerRestart routes a restart to the owner of its selected shirt,
// retaining independent button transitions for both controllers on that team.
func (m *Match) updateControllerRestart(input [2]Input) {
	team := m.Restart.Team
	previous := m.previous
	input[team], m.previous[team] = m.controllerInputs(m.originalPlayerController(team, m.Restart.Player))
	m.updateRestart(input)
	m.previous = previous
}

func (m *Match) partnerContinuePressed() bool {
	for team := range m.Teams {
		for player := range m.Teams[team].Players {
			if !m.secondaryBoundPlayer(team, player) {
				continue
			}
			current, previous := m.controllerInputs(m.originalPlayerController(team, player))
			if current.Fire && !previous.Fire {
				return true
			}
		}
	}
	return false
}

func (m *Match) aftertouchInput(input [2]Input) Vec2 {
	team := m.aftertouchTeam
	controller := m.originalPlayerController(team, m.aftertouchPlayer)
	if controller < 2 {
		return input[controller].Move
	}
	return m.cooperativeInputs[controller-2].Move
}
