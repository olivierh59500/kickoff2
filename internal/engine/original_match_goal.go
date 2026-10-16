package engine

// beginOriginalMatchGoal translates121E6/12274. The source retains its live
// update loop, net physics and moving actors throughout the scoring sequence.
func (m *Match) beginOriginalMatchGoal(team int) {
	m.Teams[team].Score++
	m.originalKickoffTeam = 1 - team
	m.originalRuleState, m.originalRuleWait = 30, 150
	m.originalMatchGoalPending = true
	m.Phase = Playing
	m.Restart = Restart{Kind: KickOff, Team: team, Player: -1}
	for side := range m.Teams {
		for index := range m.Teams[side].Players {
			p := &m.Teams[side].Players[index]
			if p.originalState == 0 {
				continue
			}
			p.originalState, p.originalDirectionHistory = 3, -1
			if side == m.originalKickoffTeam {
				p.originalState = 2
			}
		}
	}
	credited := -1
	if shooter := m.originalLastShooter; shooter >= 0 && shooter < 22 && shooter/11 == team {
		p := &m.Teams[team].Players[shooter%11]
		p.originalGoalCount++
		p.originalState, p.AnimationPhase = 35, 0
		credited = shooter % 11
	}
	// EBE8 immediately makes the rule gate ready when officials are disabled.
	// Enabled officials have a separate original actor and readiness sequence.
	m.originalSetOfficialAction(0)
	m.notice("GOAL! - "+m.Teams[team].Name, 150)
	m.emit("goal", team, credited, m.Ball.Pos)
}

func originalCelebrationPhase(p *Player) {
	if int16(p.AnimationPhase) < 0 {
		return
	}
	old := p.AnimationPhase
	p.AnimationPhase += 128
	if old>>8 >= 22 {
		p.AnimationPhase = 65535
	}
}

// updateOriginalCelebratingPlayer follows E6F6/E736. Action35's renderer B4C0
// retains the current moving step and advances even for an invisible actor.
// Action36 walks the non-taking kickoff target during its random word countdown.
func (m *Match) updateOriginalCelebratingPlayer(team, index int) (Vec2, int) {
	p := &m.Teams[team].Players[index]
	if p.originalState == 35 {
		if int16(p.AnimationPhase) >= 0 {
			originalCelebrationPhase(p)
			return p.Facing, p.originalStep
		}
		p.AnimationPhase = 0
		p.originalCelebrationWait = int16(50 + OriginalRandomWord(&m.originalRandomState)%50)
		p.originalState = 36
	} else {
		p.originalCelebrationWait--
		if p.originalCelebrationWait <= 0 {
			p.AnimationPhase, p.originalState = 0, 35
			originalCelebrationPhase(p)
			return p.Facing, p.originalStep
		}
	}
	target, _ := m.restartTacticPosition(team, index, KickOff, false, m.Ball.Pos, m.AttackDirection(team))
	move, step, _ := m.originalRunToTarget(p, target)
	return move, step
}

// updateOriginalMatchGoalRule translates30->9->1->2->36. Rule assignments
// return from the source dispatcher; a new rule never also runs that update.
func (m *Match) updateOriginalMatchGoalRule() {
	// 12274 also uses the normal first wait for an opposing practice goal;
	// its following shared9/1/2/36 sequence is handled by the practice helper.
	if m.Config.Practice && m.originalRuleState != 30 {
		return
	}
	if m.originalMatchGoalPending {
		m.originalMatchGoalPending = false
		return
	}
	switch m.originalRuleState {
	case 10:
		if m.originalActorsStopped() {
			m.originalRuleState, m.originalRuleWait = 11, 100
		}
	case 11:
		m.originalRuleWait--
		if m.originalRuleWait > 0 {
			return
		}
		for team := range m.Teams {
			for actor := range m.Teams[team].Players {
				p := &m.Teams[team].Players[actor]
				if p.originalState != 0 {
					p.originalState, p.originalDirectionHistory = 3, -1
					if team == m.originalKickoffTeam {
						p.originalState = 2
					}
				}
			}
		}
		m.originalRuleState, m.originalRuleWait = 1, 0
		m.originalSetOfficialAction(0)
		for actor := 9; actor >= 0; actor-- {
			if m.Teams[m.originalKickoffTeam].Players[actor].originalState != 0 {
				m.originalCameraTarget = m.originalKickoffTeam*11 + actor
				break
			}
		}
	case 30:
		m.originalRuleWait--
		if m.originalRuleWait <= 0 {
			m.deferOriginalCameraTarget(m.originalLastShooter)
			if shooter := m.originalLastShooter; shooter >= 0 && shooter < 22 {
				m.notice("SCORER - "+m.Teams[shooter/11].Players[shooter%11].Name, 150)
			}
			m.originalRuleState, m.originalRuleWait = 9, 150
		}
	case 9:
		m.originalRuleWait--
		if m.originalRuleWait <= 0 {
			m.originalRuleState = 1
			m.deferOriginalCameraTarget(-1)
			m.originalCenterBall()
		}
	case 1:
		if m.originalActorsStopped() && m.originalOfficialReady {
			m.originalRuleState, m.originalRuleWait = 2, 50
			m.Ball.LastTeam, m.Ball.LastPlayer = -1, -1
			m.originalBeginPendingSubstitution()
		}
	case 2:
		m.originalRuleWait--
		if m.originalRuleWait > 0 {
			return
		}
		// 126F8 literally assigns A1/B3 before choosing either side's
		// nearest kickoff actor. Do not symmetrize this source ordering.
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
		if taker := m.originalNearest(m.originalKickoffTeam, -1); taker >= 1 {
			m.Teams[m.originalKickoffTeam].Players[taker].originalState = 31
		}
		m.originalCameraTarget = -1
		m.originalClockRunning = true
		m.originalRuleState = 36
		m.originalSetOfficialAction(2)
	case 36:
		if m.Ball.LastTeam < 0 {
			return
		}
		toucher := m.Ball.LastTeam*11 + m.Ball.LastPlayer
		state := m.Teams[toucher/11].Players[toucher%11].originalState
		for team := range m.Teams {
			for index := range m.Teams[team].Players {
				p := &m.Teams[team].Players[index]
				if p.originalState != 0 {
					p.originalState, p.originalDirectionHistory = 1, -1
				}
			}
		}
		m.Teams[toucher/11].Players[toucher%11].originalState = state
		m.originalRuleState = 0
	}
}
