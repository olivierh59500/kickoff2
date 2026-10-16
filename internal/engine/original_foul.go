package engine

import "math"

// OriginalFoulState preserves the shared foul context at19D10..19D22 and the
// stoppage clock used by12560. Actor identities use team*11+playing-record.
type OriginalFoulState struct {
	Active                   bool
	Pos                      Vec2
	Team, Victim, Offender   int
	StoppageClock, LostTicks uint32
}

// originalTackleContact translates11C0A before the source's low-ball contact.
// The human caller uses distance100 and the NPC caller uses120-tackling. A
// prior ball touch skips the referee draw but still knocks the opponent down.
func (m *Match) originalTackleContact(team, index, victim, step int) {
	p, q := &m.Teams[team].Players[index], &m.Teams[victim/11].Players[victim%11]
	foul := false
	if !p.originalTackleHit && step >= 80 {
		foul = int(OriginalRandomWord(&m.originalRandomState)%100) < int(m.Referee.Attributes[3])
	}
	if foul && m.originalFoul.Active {
		return
	}
	if foul {
		m.originalFoul.Active = true
		m.originalFoul.Team, m.originalFoul.Victim, m.originalFoul.Offender = victim/11, victim, team*11+index
		m.originalFoul.Pos = Vec2{math.Floor(q.Pos.X), math.Floor(q.Pos.Y)}
		m.originalCameraTarget = victim
	}
	q.originalState, q.AnimationPhase = 14, 0
	q.originalRecoveryLimit = uint16(20+OriginalRandomWord(&m.originalRandomState)%20) << 8
	if foul && OriginalRandomWord(&m.originalRandomState)%100 <= 60 && int(OriginalRandomWord(&m.originalRandomState)%100) >= int(math.Round(q.Attributes.Resilience*100)) {
		q.originalInjuryWord++
		// MOVE.W to+24 changes only the integer word of the pace accumulator.
		q.originalPaceFixed = q.originalPaceFixed&65535 | 100<<16
	}
	m.originalBallOwner = -1
	m.Ball.TrappedBy = -1
	m.emit("tackle_hit", team, index, q.Pos)
}

// originalAdjudicateFoul follows120CA..121E4. Booking counts and weighted foul
// points are separate words. The delay and discipline draws remain in source
// order, after every actor and the current clock increment.
func (m *Match) originalAdjudicateFoul() {
	foul := &m.originalFoul
	p := &m.Teams[foul.Offender/11].Players[foul.Offender%11]
	m.originalRuleWait = 50 + int(OriginalRandomWord(&m.originalRandomState)%25)
	// 120EA publishes the offender as last contact, separately from the
	// intentional shooter pointer used for goal credit and shot aftertouch.
	m.Ball.LastTeam, m.Ball.LastPlayer = foul.Offender/11, foul.Offender%11
	roll := int(OriginalRandomWord(&m.originalRandomState) % uint16(max(1, int(m.Referee.Attributes[0]))))
	p.originalFoulPoints++
	m.originalRuleState = 6
	// 12106 compares a signed word after the increment, including wraparound.
	if roll < int(int16(p.originalFoulPoints)) {
		p.originalBookings++
		if m.Referee.Attributes[1] >= 200 || p.originalBookings == 2 {
			dismissed := 0
			for actor := range m.Teams[foul.Offender/11].Players {
				if m.Teams[foul.Offender/11].Players[actor].originalState == 0 {
					dismissed++
				}
			}
			if dismissed < 5 {
				p.originalBookings, m.originalRuleState = 2, 15
			} else if p.originalBookings == 1 {
				p.originalBookings, m.originalRuleState = 3, 16
			}
		} else if p.originalBookings == 1 {
			m.originalRuleState = 16
		} else {
			// 12126 branches to12180 for a third or later booking unless
			// this referee's direct-dismissal attribute selected12128.
			p.originalFoulPoints += uint16(m.Referee.Attributes[2])
		}
		if m.originalRuleState == 16 {
			p.originalFoulPoints += uint16(m.Referee.Attributes[1])
		}
	} else {
		p.originalFoulPoints += uint16(m.Referee.Attributes[2])
	}
	if m.originalRuleState == 6 {
		m.originalCameraTarget = foul.Victim
	}
	foul.StoppageClock = m.originalClock
	m.notice("FOUL BY "+p.Name, 400)
	m.emit("foul", foul.Offender/11, foul.Offender%11, foul.Pos)
}

func (m *Match) originalStageFreeKick() {
	foul := &m.originalFoul
	victim := &m.Teams[foul.Victim/11].Players[foul.Victim%11]
	state := victim.originalState
	for team := range m.Teams {
		for index := range m.Teams[team].Players {
			p := &m.Teams[team].Players[index]
			if p.originalState != 0 {
				p.originalState, p.originalDirectionHistory = 27, -1
				if team == foul.Team {
					p.originalState = 26
				}
			}
		}
	}
	victim.originalState = state
	ball := originalBallState(m.Ball, m.ballCurve)
	ball.X = int32(foul.Pos.X)<<16 | int32(uint16(ball.X))
	ball.Y = int32(foul.Pos.Y)<<16 | int32(uint16(ball.Y))
	ball.Height, ball.VelocityX, ball.VelocityY, ball.Lift = 0, 0, 0, 0
	m.Ball.applyOriginalState(ball)
	m.originalBallDrawEnabled = true
	m.Ball.TrappedBy = -1
	m.originalGoalFrame.OutKind = 0
	m.originalRuleState = 7
}

func (m *Match) updateOriginalFoulRule() {
	switch m.originalRuleState {
	case 0, 20:
		if m.originalFoul.Active {
			m.originalAdjudicateFoul()
		}
	case 6:
		m.originalRuleWait--
		if m.originalRuleWait <= 0 {
			m.originalStageFreeKick()
		}
	case 15, 16:
		m.originalRuleWait--
		if m.originalRuleWait <= 0 {
			red := m.originalRuleState == 15
			height := int32(uint16(originalBallState(m.Ball, m.ballCurve).Height))
			m.originalStageFreeKick()
			ball := originalBallState(m.Ball, m.ballCurve)
			ball.Height = height
			m.Ball.applyOriginalState(ball)
			p := &m.Teams[m.originalFoul.Offender/11].Players[m.originalFoul.Offender%11]
			p.originalState = 34
			m.originalCameraTarget = m.originalFoul.Offender
			m.originalRuleState, m.originalRuleWait = 18, 150
			m.originalSetOfficialAction(5)
			if red {
				m.originalRuleState = 19
				m.originalSetOfficialAction(6)
				m.notice("RED CARD! "+p.Name, 30000)
				m.emit("red_card", m.originalFoul.Offender/11, m.originalFoul.Offender%11, p.Pos)
			} else {
				p.YellowCards = 1
				m.notice("YELLOW CARD! "+p.Name, 300)
				m.emit("yellow_card", m.originalFoul.Offender/11, m.originalFoul.Offender%11, p.Pos)
			}
		}
	case 17, 18, 19:
		victim := &m.Teams[m.originalFoul.Victim/11].Players[m.originalFoul.Victim%11]
		victim.originalState, victim.originalDirectionHistory, victim.AnimationPhase = 14, -1, 0xA00
		if m.originalRuleState == 17 {
			offender := &m.Teams[m.originalFoul.Offender/11].Players[m.originalFoul.Offender%11]
			if int(math.Floor(offender.Pos.X)) > 140 {
				m.originalCameraHoldX = int32(math.Round(offender.Pos.X * originalFixedScale))
				m.originalCameraHoldY = int32(math.Round(offender.Pos.Y * originalFixedScale))
			}
			if int(math.Floor(offender.Pos.X)) <= 140 {
				m.originalCameraTarget = -3
			}
			if int(math.Floor(offender.Pos.X)) < -60 {
				m.originalRemoveDepartingPlayer(offender)
				offender.SentOff = true
				m.originalCameraTarget = m.originalFoul.Victim
				m.originalRuleState, m.originalRuleWait = 7, 50
				m.MessageTicks = 1
			}
		} else if !m.originalOfficialReady {
			m.originalRuleWait = 50
		} else {
			// Preserve the stored signed-word wrap while deciding expiry from
			// the prior value, as the source's direct subtraction branch does.
			previousWait := int16(m.originalRuleWait)
			m.originalRuleWait = int(previousWait - 1)
			if previousWait <= 1 {
				if m.originalRuleState == 19 {
					p := &m.Teams[m.originalFoul.Offender/11].Players[m.originalFoul.Offender%11]
					p.originalState, p.originalDirectionHistory = 4, -1
					m.originalRuleState = 17
					// The completed red gesture publishes integer coordinates
					// before departure, retaining the camera hold's fractions.
					m.originalCameraHoldX = int32(math.Floor(p.Pos.X))<<16 | int32(uint16(m.originalCameraHoldX))
					m.originalCameraHoldY = int32(math.Floor(p.Pos.Y))<<16 | int32(uint16(m.originalCameraHoldY))
				} else {
					m.originalCameraTarget = m.originalFoul.Victim
					m.originalRuleState, m.originalRuleWait = 7, 50
				}
				m.originalSetOfficialAction(2)
			}
		}
	case 7:
		p := &m.Teams[m.originalFoul.Victim/11].Players[m.originalFoul.Victim%11]
		if p.originalState != 1 {
			p.touchCooldown = 15
		} else if p.originalInjuryWord != 0 {
			m.beginOriginalInjurySubstitution()
		} else {
			m.originalDispatchFoulRestart()
		}
	case 8:
		foul := &m.originalFoul
		if m.Ball.LastTeam == foul.Victim/11 && m.Ball.LastPlayer == foul.Victim%11 {
			p := &m.Teams[foul.Victim/11].Players[foul.Victim%11]
			state := p.originalState
			for team := range m.Teams {
				for index := range m.Teams[team].Players {
					q := &m.Teams[team].Players[index]
					if q.originalState != 0 {
						q.originalState, q.originalDirectionHistory = 1, -1
					}
				}
			}
			p.originalState = state
			m.originalCameraTarget, m.originalRuleState, foul.Active = -1, 0, false
			foul.LostTicks += m.originalClock - foul.StoppageClock
			foul.StoppageClock = 0
		}
	case 38, 39, 40, 41, 42, 43, 44:
		m.updateOriginalInjurySubstitution()
	case 31, 32, 33, 34, 46, 47:
		m.updateOriginalFoulRestartRule()
	}
}

// updateOriginalFreeKickActor translates E48C/E524. The two nearest runners
// approach the foul position; other actors retain their ordinary tactical run.
// Both sides stay outside their source60/72 or120/144 distance bands.
func (m *Match) updateOriginalFreeKickActor(team, index, closest, second int, input Input, fixed bool) (Vec2, int) {
	p := &m.Teams[team].Players[index]
	if !fixed && uint16(int16(math.Floor(p.Pos.X))) >= 912 {
		// E49E/E536 write the whole A2/A4 longword and keep the old
		// movement for this frame before action40 takes over.
		p.originalRecoveryLimit, p.originalOpaqueWords[3] = 0, 3600
		p.originalState, p.originalDirectionHistory = 40, -1
		return p.Facing, p.originalStep
	}
	move, step := Vec2{}, 0
	if fixed {
		direction := -1
		if input.Move.Len() > .05 {
			direction = OriginalDirectionIndex(input.Move)
		}
		move, step = originalHumanRun(p, direction, originalPace(p))
	} else if index == closest || index == second {
		move, step, _ = m.originalRunToTarget(p, m.originalFoul.Pos)
	} else {
		move, step = m.originalTacticRun(team, index)
	}
	inner, outer := int32(3600), int32(5184)
	if p.originalState == 27 {
		inner, outer = 14400, 20736
	}
	distance := originalDistanceSquared(p.Pos, m.originalFoul.Pos)
	if distance < inner {
		p.Facing = OriginalDirection(OriginalDirectionIndex(Vec2{math.Floor(m.originalFoul.Pos.X) - math.Floor(p.Pos.X), math.Floor(m.originalFoul.Pos.Y) - math.Floor(p.Pos.Y)}) + 4)
		return p.Facing, originalPace(p)
	}
	if distance <= outer {
		// B91A faces the ball directly; it does not retain B832 hysteresis
		// after the preceding outward step changed the actor's heading.
		p.Facing = m.originalBallFacing(p)
		p.originalDirectionHistory = -1
		return Vec2{}, 0
	}
	return move, step
}

// E75A moves an outside participant horizontally back into the pitch, also
// enforcing the saved foul-distance threshold while the stoppage continues.
func (m *Match) updateOriginalFreeKickRetreat(p *Player) (Vec2, int) {
	threshold := int32(uint32(p.originalRecoveryLimit)<<16 | uint32(uint16(p.originalOpaqueWords[3])))
	if originalDistanceSquared(p.Pos, m.originalFoul.Pos) >= threshold && uint16(int16(math.Floor(p.Pos.X))) < 912 {
		return Vec2{}, 0
	}
	direction := 2
	if int(math.Floor(p.Pos.X)) >= 456 {
		direction = 6
	}
	p.Facing = OriginalDirection(direction)
	return p.Facing, originalPace(p)
}
