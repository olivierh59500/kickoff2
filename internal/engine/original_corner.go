package engine

import "math"

// beginOriginalCorner translates0x124c0..0x1255e. Corner preparation keeps
// the ball free while actors first walk to the special tactic targets.
func (m *Match) beginOriginalCorner(team int) {
	m.originalCornerTeam = team
	for side := range m.Teams {
		for index := range m.Teams[side].Players {
			p := &m.Teams[side].Players[index]
			if p.originalState == 0 {
				continue
			}
			p.originalState, p.originalDirectionHistory = 62, -1
			if side == team {
				correctEnd := m.AttackDirection(team) < 0 && m.originalGoalFrame.OutY < 696 || m.AttackDirection(team) > 0 && m.originalGoalFrame.OutY >= 696
				if !m.Config.Practice || correctEnd {
					p.originalState = 61
				}
			}
		}
	}
	m.originalRuleState, m.originalRuleWait = 37, 20
	m.Restart = Restart{Kind: Corner, Team: team, Player: -1, Pos: Vec2{float64(m.originalGoalFrame.OutX), float64(m.originalGoalFrame.OutY)}}
	m.notice("CORNER KICK", 350)
	m.emit("whistle", team, -1, m.Restart.Pos)
}

// originalRunToTarget is C736: full pace until the integer squared distance
// is at most16, then face the ball and invalidate heading history.
func (m *Match) originalRunToTarget(p *Player, target Vec2) (Vec2, int, bool) {
	if originalDistanceSquared(p.Pos, target) > 16 {
		p.Facing = originalTargetHeading(p, target)
		return p.Facing, originalPace(p), false
	}
	p.Facing = m.originalBallFacing(p)
	p.originalDirectionHistory = -1
	return Vec2{}, 0, true
}

func (m *Match) updateOriginalCornerActor(team, index int, in Input) (Vec2, int) {
	p := &m.Teams[team].Players[index]
	switch p.originalState {
	case 61, 62:
		target, _ := m.restartTacticPosition(team, index, Corner, p.originalState == 61, Vec2{float64(m.originalGoalFrame.OutX), float64(m.originalGoalFrame.OutY)}, m.AttackDirection(team))
		move, step, _ := m.originalRunToTarget(p, target)
		return move, step
	case 21, 22:
		minimum, maximum := int32(5184), int32(7056)
		if p.originalState == 22 {
			minimum, maximum = 14400, 20736
		}
		distance := originalDistanceSquared(p.Pos, m.Ball.Pos)
		if distance < minimum {
			towards := OriginalDirectionIndex(Vec2{math.Floor(m.Ball.Pos.X) - math.Floor(p.Pos.X), math.Floor(m.Ball.Pos.Y) - math.Floor(p.Pos.Y)})
			p.Facing = OriginalDirection(towards + 4)
			return p.Facing, originalPace(p)
		}
		if distance <= maximum {
			p.Facing = OriginalDirection(OriginalDirectionIndex(Vec2{math.Floor(m.Ball.Pos.X) - math.Floor(p.Pos.X), math.Floor(m.Ball.Pos.Y) - math.Floor(p.Pos.Y)}))
			p.originalDirectionHistory = -1
			return Vec2{}, 0
		}
		return p.Facing, p.originalStep
	case 23:
		move, step, arrived := m.originalRunToTarget(p, p.originalRestartTarget)
		if arrived {
			p.originalState, p.originalDirectionHistory = 24, -1
			if m.Config.Human[team] {
				p.originalState = 25
			}
			m.originalCornerCountdown, m.originalCornerHeld = 150, 0
			m.originalBallOwner = -1
			m.originalBallDrawEnabled = true
		}
		return move, step
	case 24:
		m.originalCornerCountdown--
		if m.originalCornerCountdown <= 0 {
			move, step, ready := m.originalCornerApproach(team, index)
			if ready {
				choice := int(OriginalRandomWord(&m.originalRandomState) % 9)
				power := 16 + int(OriginalRandomWord(&m.originalRandomState)%5)
				m.launchOriginalCorner(team, index, choice, power-4, false)
			}
			return move, step
		}
		return p.Facing, p.originalStep
	case 25:
		p.originalDirectionHistory = -1
		m.originalCornerCountdown--
		if m.originalCornerCountdown == 50 {
			m.originalCornerSelecting = true
			m.originalCornerHeld = 0
			m.beginOriginalCornerSelectorVideo()
			selectorInput := in
			if m.originalHasQueuedSelectorInputs {
				controller := m.originalPlayerController(team, index)
				selectorInput = m.originalQueuedSelectorInputs[controller]
				if len(m.originalQueuedModalPolls) != 0 && m.originalQueuedModalController == controller {
					// The source blocks here while hardware keeps sampling. These
					// are declared external input samples, not observed game state.
					m.updateOriginalCornerSelector(in)
					for _, poll := range m.originalQueuedModalPolls {
						m.sampleOriginalModalInput(controller, poll)
						m.updateOriginalCornerSelector(poll)
						if !m.originalCornerSelecting {
							break
						}
					}
					if m.originalCornerSelecting {
						m.sampleOriginalModalInput(controller, selectorInput)
					}
				}
			}
			m.updateOriginalCornerSelector(selectorInput)
			return p.Facing, p.originalStep
		}
		if m.originalCornerCountdown <= 0 {
			move, step, ready := m.originalCornerApproach(team, index)
			if ready {
				m.launchOriginalCorner(team, index, m.Restart.CornerChoice, m.originalCornerHeld, true)
				p.AnimationPhase = 0
			}
			return move, step
		}
		if in.Fire {
			m.originalCornerHeld++
		}
		return p.Facing, p.originalStep
	}
	return Vec2{}, 0
}

// OriginalCornerSelectorActive exposes the blocking D69A selector to a host.
// Match.Update polls its input while freezing the source match update kernel.
func (m *Match) OriginalCornerSelectorActive() bool { return m.originalCornerSelecting }

// QueueOriginalCornerSelectorInputs supplies a separate host input sample for
// D69A's reread if the selector starts during the next Update. Ordinary actor
// inputs are unchanged; the sample expires at that Update's completion.
func (m *Match) QueueOriginalCornerSelectorInputs(input [4]Input) {
	m.originalQueuedSelectorInputs, m.originalHasQueuedSelectorInputs = input, true
}

// QueueOriginalModalInputPolls retains actual sampled joystick stimuli during a
// blocking corner. Polls update input history inside that one game iteration;
// they do not run ball/player logic or advance match clocks. Confirmation is
// still supplied separately through QueueOriginalCornerSelectorInputs.
func (m *Match) QueueOriginalModalInputPolls(controller int, polls []Input) bool {
	if controller < 0 || controller >= 4 || len(polls) > 128 {
		return false
	}
	m.originalQueuedModalController = controller
	m.originalQueuedModalPolls = append(m.originalQueuedModalPolls[:0], polls...)
	return true
}

func (m *Match) sampleOriginalModalInput(controller int, input Input) {
	// 16600 stores the old sampled direction before polling the hardware.
	// Explicit modal samples represent additional host ticks after the outer
	// world call; its earlier one-tick guard no longer covers this continuation.
	m.originalPALHistorySampled = false
	m.updateOriginalInputHistory()
	if controller < 2 {
		m.controls[controller] = input
	} else {
		m.cooperativeInputs[controller-2] = input
	}
}

func (m *Match) updateOriginalCornerSelector(in Input) {
	choice := 0
	if in.CornerChoice != nil {
		choice = clampInt(*in.CornerChoice, 0, 8)
	} else if in.Move.Len() > .05 {
		choice = OriginalDirectionIndex(in.Move) + 1
	}
	m.Restart.CornerChoice = choice
	m.recordOriginalCornerSelectorChoice(choice)
	if in.Fire {
		m.originalCornerSelecting = false
		m.originalBallDrawEnabled = true
		m.finishOriginalCornerSelectorVideo()
		m.Restart.Armed = true
		// The caller at1055E counts the confirming held button immediately.
		m.originalCornerHeld = 1
	}
}

func (m *Match) originalCornerApproach(team, index int) (Vec2, int, bool) {
	p := &m.Teams[team].Players[index]
	if m.originalBallOwner == team*11+index {
		return Vec2{}, 0, true
	}
	if p.originalBallDistance <= 64 && int(m.Ball.Height) <= 12 {
		m.originalBallOwner = team*11 + index
		p.originalOwnerTicks = 10
		return Vec2{}, 0, true
	}
	p.Facing = m.originalChaseHeading(p)
	return p.Facing, originalPace(p), false
}

func (m *Match) launchOriginalCorner(team, index, choice, charge int, human bool) {
	p := &m.Teams[team].Players[index]
	state := originalBallState(m.Ball, m.ballCurve)
	launch := OriginalCornerState(m.Ball.Pos, choice, charge)
	state.VelocityX, state.VelocityY, state.Lift, state.Curve = launch.VelocityX, launch.VelocityY, launch.Lift, launch.Curve
	m.Ball.applyOriginalState(state)
	m.ballCurve = state.Curve
	m.originalBallOwner, m.Ball.TrappedBy = -1, -1
	m.Ball.LastTeam, m.Ball.LastPlayer = team, index
	m.originalLastShooter = team*11 + index
	p.touchCooldown, p.lastTouch = 15, m.Tick
	if human {
		for side := range m.Teams {
			for actor := range m.Teams[side].Players {
				q := &m.Teams[side].Players[actor]
				if q.originalState != 0 && (side == team || !m.Config.Practice) {
					q.originalState = 1
				}
			}
		}
	}
	m.Restart.Armed = false
	m.emit("corner", team, index, m.Ball.Pos)
}

func (m *Match) updateOriginalCornerRule() {
	if m.originalRuleState == 4 {
		if m.Ball.LastTeam*11+m.Ball.LastPlayer != m.originalCornerTaker {
			return
		}
		for team := range m.Teams {
			for index := range m.Teams[team].Players {
				p := &m.Teams[team].Players[index]
				if p.originalState != 0 {
					p.originalState, p.originalDirectionHistory = 1, -1
				}
			}
		}
		m.originalRuleState = 0
		return
	}
	if m.originalRuleState != 37 {
		return
	}
	// 12780 uses the shared 11D76 roster scan, including A's reserves
	// and ending before B's last two playing records.
	if !m.originalActorsStopped() {
		return
	}
	m.originalRuleWait--
	if m.originalRuleWait == 10 {
		state := originalBallState(m.Ball, m.ballCurve)
		x, y := int32(8), int32(8)
		if m.originalGoalFrame.OutX >= 456 {
			x = 904
		}
		if m.originalGoalFrame.OutY > 0 {
			y = 1384
		}
		state.X, state.Y = x<<16|int32(uint16(state.X)), y<<16|int32(uint16(state.Y))
		state.Height, state.VelocityX, state.VelocityY, state.Lift = 0, 0, 0, 0
		m.Ball.applyOriginalState(state)
	}
	if m.originalRuleWait > 0 {
		return
	}
	team := m.originalCornerTeam
	taker := m.originalNearest(team, -1)
	if taker < 0 {
		return
	}
	for side := range m.Teams {
		for index := range m.Teams[side].Players {
			p := &m.Teams[side].Players[index]
			if p.originalState == 0 {
				continue
			}
			p.originalState, p.originalDirectionHistory = 22, -1
			if side == team {
				p.originalState = 21
			}
		}
	}
	p := &m.Teams[team].Players[taker]
	p.originalState, p.originalDirectionHistory = 23, -1
	p.originalRestartTarget = Vec2{-54, -60}
	if m.originalGoalFrame.OutX >= 456 {
		p.originalRestartTarget.X = 966
	}
	if m.originalGoalFrame.OutY > 0 {
		p.originalRestartTarget.Y = 1452
	}
	m.originalCornerTaker, m.Restart.Player = team*11+taker, taker
	m.Ball.LastTeam, m.Ball.LastPlayer = -1, -1
	m.originalRuleState = 4
}
