package engine

// UserTacticPreviewPosition applies the original preview convention to imported
// tactic bytes while retaining the selected embedded formation as the fallback.
func UserTacticPreviewPosition(config Config, team, player int) Vec2 {
	state := OriginalAssignedTactics(config, team)
	id := state.Slots[state.Active]
	if id < OriginalCustomTacticID {
		return TacticPreviewPosition(originalFormationFromIndex(id), player)
	}
	tactic, err := DecodeTactic(originalAssignedTacticData(config, team, state.Active))
	if err != nil {
		return TacticPreviewPosition(config.Formation[team], player)
	}
	position := tactic.Targets[clampInt(player, 1, 10)-1][15]
	position.Y = PitchHeight - position.Y
	return position
}

// tacticPosition applies an imported original tactic's normal-play zone table.
// Invalid external bytes fall back to the embedded tactic rather than changing
// the native coordinate or zone conventions.
func (m *Match) tacticPosition(team, player int, ball Vec2, direction int) Vec2 {
	if player >= 0 && player < 11 && m.Teams[team].Players[player].originalHasTargets {
		zone := clampInt(int(ball.X/304), 0, 2)*4 + clampInt(int(ball.Y/348), 0, 3)
		if direction < 0 {
			zone = 11 - zone
		}
		return m.Teams[team].Players[player].originalTargets[zone]
	}
	if len(m.Config.UserTactics[team]) == 0 {
		return TacticPosition(m.Teams[team].Formation, player, ball, direction)
	}
	tactic, err := DecodeTactic(m.Config.UserTactics[team])
	if err != nil {
		return TacticPosition(m.Teams[team].Formation, player, ball, direction)
	}
	zone := clampInt(int(ball.X/304), 0, 2)*4 + clampInt(int(ball.Y/348), 0, 3)
	if direction < 0 {
		zone = 11 - zone
	}
	target := tactic.Targets[clampInt(player, 1, 10)-1][zone]
	if direction < 0 {
		target = Vec2{PitchWidth - target.X, PitchHeight - target.Y}
	}
	return target
}

func (m *Match) restartTacticPosition(team, player int, kind RestartKind, taking bool, corner Vec2, direction int) (Vec2, bool) {
	if player >= 0 && player < 11 && m.Teams[team].Players[player].originalHasTargets {
		zone := -1
		switch kind {
		case KickOff:
			zone = 12
			if !taking {
				zone++
			}
		case GoalKick:
			zone = 14
			if !taking {
				zone++
			}
		case Corner:
			zone = 16
			if taking {
				zone++
			}
			if corner.X >= PitchWidth/2 {
				zone += 2
			}
		}
		if zone >= 0 {
			return m.Teams[team].Players[player].originalTargets[zone], true
		}
	}
	if len(m.Config.UserTactics[team]) == 0 {
		return RestartTacticPosition(m.Teams[team].Formation, player, kind, taking, corner, direction)
	}
	tactic, err := DecodeTactic(m.Config.UserTactics[team])
	if err != nil {
		return RestartTacticPosition(m.Teams[team].Formation, player, kind, taking, corner, direction)
	}
	zone := -1
	switch kind {
	case KickOff:
		zone = 12
		if !taking {
			zone = 13
		}
	case GoalKick:
		zone = 14
		if !taking {
			zone = 15
		}
	case Corner:
		zone = 16
		if taking {
			zone = 17
		}
		if corner.X >= PitchWidth/2 {
			zone += 2
		}
	}
	if zone < 0 {
		return Vec2{}, false
	}
	target := tactic.Targets[clampInt(player, 1, 10)-1][zone]
	if direction < 0 {
		target.Y = PitchHeight - target.Y
		if zone < 16 {
			target.X = PitchWidth - target.X
		}
	}
	return target, true
}
