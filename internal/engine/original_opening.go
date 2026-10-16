package engine

// beginOriginalOpening translates 11E1C/14832: all playing actors enter from
// the left and line up on the two rows at680/712. MOVE.W preserves coordinate
// fractions; a fresh native allocation begins with zero fractions.
func (m *Match) beginOriginalOpening() {
	m.Phase, m.originalRuleState, m.originalRuleWait = Playing, 10, 0
	m.originalClockRunning = false
	m.ClockTicks, m.originalClock = 0, 0
	m.originalKickoffTeam = m.originalPeriod.InitialKickoff
	m.originalOpeningSoundPending = true
	m.Restart = Restart{Kind: KickOff, Team: m.originalKickoffTeam, Player: -1, Pos: Vec2{456, 696}}
	m.originalCenterBall()
	m.originalCameraTarget = -2
	m.originalCameraFixedX, m.originalCameraFixedY = 160<<16, 696<<16
	for team := range m.Teams {
		for actor := range m.Teams[team].Players {
			p := &m.Teams[team].Players[actor]
			x, y := 80+actor*16, 680
			if m.AttackDirection(team) < 0 {
				y = 1392 - y
			}
			p.originalEntryTarget = Vec2{float64(x), float64(y)}
			p.Pos = Vec2{float64(x - 250), float64(y)}
			p.originalState, p.originalDirectionHistory, p.originalStep = 33, -1, originalPace(p)
		}
	}
	m.Events = m.Events[:0]
	m.Message, m.MessageTicks = "", 0
}

// GoalReplayAvailable exposes the visible goal interval without replacing the
// source's live rule loop with a frozen native Goal phase.
func (m *Match) GoalReplayAvailable() bool {
	return m.Phase == Goal || m.originalRuleState == 30 || m.originalRuleState == 9
}

// updateOriginalEnteringPlayer retains the identical E688/141E0 target handler
// for footballers and keepers. C736 replaces the provisional step100 with the
// actor's full pace. Arrival faces the attacking end, regardless of the ball.
func (m *Match) updateOriginalEnteringPlayer(team int, p *Player) (Vec2, int) {
	move, step, arrived := m.originalRunToTarget(p, p.originalEntryTarget)
	if arrived {
		direction := 4
		if m.AttackDirection(team) < 0 {
			direction = 0
		}
		p.Facing = OriginalDirection(direction)
	}
	return move, step
}
