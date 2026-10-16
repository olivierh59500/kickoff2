package engine

import "math"

// ControlledActor resolves the selected roster slot to team*11+shirt. Source
// replacement can select an opposing actor without transferring its input.
func (team Team) ControlledActor(owner int) int {
	if owner < 0 || owner > 1 || team.Controlled < 0 || team.Controlled > 10 {
		return -1
	}
	if team.ControlledOpposing {
		owner = 1 - owner
	}
	return owner*11 + team.Controlled
}

func (team *Team) selectOwnPlayer(player int) {
	team.Controlled, team.ControlledOpposing = player, false
}

func (team Team) primaryControls(player int) bool {
	return !team.ControlledOpposing && team.Controlled == player
}

func (m *Match) selectControlledActor(owner, actor int) {
	if actor < 0 || actor >= 22 {
		m.Teams[owner].selectOwnPlayer(-1)
		return
	}
	m.Teams[owner].Controlled = actor % 11
	m.Teams[owner].ControlledOpposing = actor/11 != owner
}

func (m *Match) originalRecordHasController(team, player int) bool {
	if m.Config.HasSquadBindings[team] {
		_, bound := m.explicitPlayerController(team, player)
		return bound
	}
	return m.boundPrimaryPlayer(team) == player || m.boundPartnerPlayer(team) == player
}

// originalSelectionMarkerOperands preserves the two signed words left for
// C7D4 by the completed marker path.
func (m *Match) originalSelectionMarkerOperands(team int, current *Player) (x, y int16, known bool) {
	partner := m.PartnerPlayer(team)
	if partner >= 0 {
		p := &m.Teams[team].Players[partner]
		if !p.originalVisible {
			if current.originalVisible {
				// 14BBC retains D0 when the active partner's marker was clipped.
				// It therefore leaks the low word of that source record pointer into
				// C7D4. All 85 continuous v1.6e traces expose these same two roster
				// bases. Preserve only the leaked word; no guest pointer is stored.
				base := [2]uint16{0x8fd6, 0xa3a4}[team]
				shift := (int(math.Floor(current.Pos.X)) - int(m.originalCameraX>>16) - 8) & 15
				return int16(base + uint16(partner*0x186)), int16(uint32(0xffff0000) >> uint(shift)), true
			}
			return 0, 0, false
		}
		shift := (int(math.Floor(p.Pos.X)) - int(m.originalCameraX>>16) - 8) & 15
		return int16(shift * 4), int16(uint32(0xfc3f0000) >> uint(shift)), true
	}
	shift := (int(math.Floor(current.Pos.X)) - int(m.originalCameraX>>16) - 8) & 15
	return 0, int16(uint32(0xffff0000) >> uint(shift)), true
}

// originalDirectionIndexWords translates B8C8's signed word arithmetic. The
// intermediate ASL operations can overflow for retained marker words even when
// ordinary pitch coordinates would produce the same result as the float helper.
func originalDirectionIndexWords(targetX, targetY, originX, originY int16) int {
	d0, d1 := targetX-originX, targetY-originY
	d2, d3 := d0, d1
	if d2 < 0 {
		d2 = int16(-int32(d2))
	}
	if d3 < 0 {
		d3 = int16(-int32(d3))
	}
	d2 = int16(uint16(d2) << 1)
	if d2 < d3 {
		if d1 > 0 {
			return 4
		}
		return 0
	}
	d2 = int16(int32(d2) >> 1)
	d3 = int16(uint16(d3) << 1)
	if d3 < d2 {
		if d0 < 0 {
			return 6
		}
		return 2
	}
	direction := 1
	if d0 < 0 {
		direction = 7
	}
	if d1 > 0 {
		direction ^= 2
	}
	return direction
}
