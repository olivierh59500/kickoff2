package engine

// beginOriginalPracticeGoal is the practice branch at12252. The original
// keeps its live update kernel running while actors walk the kickoff targets.
func (m *Match) beginOriginalPracticeGoal() {
	m.Teams[0].Score++
	m.originalKickoffTeam = 0
	m.originalRuleState, m.originalRuleWait = 9, 300
	m.originalPracticeGoalPending = true
	for team := range m.Teams {
		for index := range m.Teams[team].Players {
			p := &m.Teams[team].Players[index]
			if p.originalState != 0 {
				p.originalState, p.originalDirectionHistory = 3, -1
				if team == 0 {
					p.originalState = 2
				}
			}
		}
	}
	m.notice(m.Teams[0].Name+" SCORES!", 150)
	m.emit("goal", 0, m.Ball.LastPlayer, m.Ball.Pos)
}

func (m *Match) originalActorsStopped() bool {
	// 11D76 scans22 consecutive records, starting at Team A. Each squad
	// owns13 records, so this includes A's two reserves and B's first nine
	// playing records. B's final two runners do not delay the source wait.
	for record := 0; record < 22; record++ {
		var p *Player
		if record < 11 {
			p = &m.Teams[0].Players[record]
		} else if record < 13 {
			p = &m.Teams[0].Bench[record-11]
		} else {
			p = &m.Teams[1].Players[record-13]
		}
		if p.originalState != 0 && p.originalStep&0xfff0 != 0 {
			return false
		}
	}
	return true
}

// originalCenterBall translates11CC4. Height is the raw long32, not32pixels;
// the two small velocity words consume exactly two original random draws.
func (m *Match) originalCenterBall() {
	state := originalBallState(m.Ball, m.ballCurve)
	state.X, state.Y, state.Height = 456<<16, 696<<16, 32
	state.VelocityX = int32(OriginalRandomWord(&m.originalRandomState)%2000) - 1000
	state.VelocityY = int32(OriginalRandomWord(&m.originalRandomState)%2000) - 1000
	state.Lift = 0
	m.Ball.applyOriginalState(state)
	m.originalGoalFrame.OutKind = 0
	m.originalBallDrawEnabled = true
	m.originalCameraTarget = -1
}

func (m *Match) updateOriginalPracticeGoalRule() {
	if !m.Config.Practice {
		return
	}
	if m.originalPracticeGoalPending {
		// The goal's initial rule9 assignment returns from12252; its wait
		// starts decrementing on the following complete match update.
		m.originalPracticeGoalPending = false
		return
	}
	switch m.originalRuleState {
	case 9:
		m.originalRuleWait--
		if m.originalRuleWait <= 0 {
			m.originalRuleState = 1
			m.originalCenterBall()
		}
	case 1:
		if m.originalActorsStopped() {
			m.originalRuleState, m.originalRuleWait = 2, 50
			m.Ball.LastTeam, m.Ball.LastPlayer = -1, -1
			m.originalBeginPendingSubstitution()
		}
	case 2:
		m.originalRuleWait--
		if m.originalRuleWait > 0 {
			return
		}
		for team := range m.Teams {
			for index := range m.Teams[team].Players {
				p := &m.Teams[team].Players[index]
				if p.originalState != 0 {
					p.originalState, p.originalDirectionHistory = 3, -1
					if team == 0 {
						p.originalState = 1
					}
				}
			}
		}
		taker := m.originalNearest(0, -1)
		if taker >= 1 {
			m.Teams[0].Players[taker].originalState = 31
		}
		m.originalClockRunning = true
		m.originalRuleState = 36
	case 36:
		if m.Ball.LastTeam >= 0 {
			m.originalRuleState = 0
		}
	}
}
