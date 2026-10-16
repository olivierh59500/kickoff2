package engine

import "math"

// beginOriginalThrowIn translates rule21 at0x12304. The crossing coordinate
// remains separate from the freely moving ball while the thrower approaches.
func (m *Match) beginOriginalThrowIn() {
	team := 1 - m.Ball.LastTeam
	if m.Config.Practice || team < 0 || team > 1 {
		team = 0
	}
	// 1230C tests the receiving team's input pointer. A CPU throw always
	// consumes this draw, including tied scores and rejected tactic changes.
	if !m.Config.Human[team] && OriginalRandomWord(&m.originalRandomState)%100 <= 25 {
		m.originalAutomaticTactic(team)
	}
	closest, second := m.originalNearest(team, -1), -1
	if closest >= 0 {
		second = m.originalNearest(team, closest)
	}
	if closest < 1 {
		return
	}
	m.originalThrower = team*11 + closest
	m.originalGoalFrame.OutY = int16(clamp(float64(m.originalGoalFrame.OutY), 16, 1376))
	for side := range m.Teams {
		for index := range m.Teams[side].Players {
			p := &m.Teams[side].Players[index]
			if p.originalState == 0 {
				continue
			}
			p.originalDirectionHistory = -1
			if side == team {
				p.originalState = 5
				if index == closest {
					p.originalState = 9
				} else if p.originalBallDistance < 129600 || index == second {
					p.originalState = 7
				}
			} else if !m.Config.Practice {
				p.originalState = 6
				if p.originalBallDistance < 129600 || index == m.originalNearest(side, -1) || index == m.originalNearest(side, m.originalNearest(side, -1)) {
					p.originalState = 8
				}
			}
		}
	}
	m.originalRuleState, m.originalRuleWait = 3, 400
	m.Restart = Restart{Kind: ThrowIn, Team: team, Player: closest, Pos: Vec2{float64(m.originalGoalFrame.OutX), float64(m.originalGoalFrame.OutY)}}
	m.notice("THROW IN", 150)
	m.emit("whistle", team, closest, m.Restart.Pos)
}

func (m *Match) originalTacticRun(team, index int) (Vec2, int) {
	p := &m.Teams[team].Players[index]
	target := m.tacticPosition(team, index, m.Ball.Pos, m.AttackDirection(team))
	distance := originalDistanceSquared(p.Pos, target)
	if distance <= 16 {
		p.originalDirectionHistory = -1
		p.Facing = m.originalBallFacing(p)
		return Vec2{}, 0
	}
	step := originalPace(p)
	if distance <= 22500 {
		step >>= 1
		if distance <= 10000 {
			step >>= 1
		}
	}
	p.Facing = originalTargetHeading(p, target)
	return p.Facing, step
}

func (m *Match) updateOriginalThrowActor(team, index int, in Input, controlled bool) (Vec2, int) {
	p := &m.Teams[team].Players[index]
	crossing := Vec2{float64(m.originalGoalFrame.OutX), float64(m.originalGoalFrame.OutY)}
	if p.originalState == 29 {
		if m.Ball.LastTeam != team || m.Ball.LastPlayer != index {
			p.originalState, p.originalDirectionHistory, p.AnimationPhase = 1, -1, 0
		}
		return Vec2{}, 0
	}
	if p.originalState == 10 {
		return m.updateOriginalThrower(team, index, in, controlled)
	}
	if p.originalState == 9 {
		if m.Config.Human[team] && m.boundPrimaryPlayer(team) < 0 {
			m.Teams[team].selectOwnPlayer(index)
		}
		if crossing.X > 0 {
			crossing.X += 8
		} else {
			crossing.X -= 4
		}
		if originalDistanceSquared(p.Pos, crossing) > 16 {
			p.Facing = originalTargetHeading(p, crossing)
			return p.Facing, originalPace(p)
		}
		state := originalBallState(m.Ball, m.ballCurve)
		state.X = int32(math.Floor(p.Pos.X))<<16 | int32(uint16(state.X))
		state.Y = int32(math.Floor(p.Pos.Y))<<16 | int32(uint16(state.Y))
		state.Height = int32(uint16(state.Height))
		state.VelocityX, state.VelocityY, state.Lift = 0, 0, 0
		m.Ball.applyOriginalState(state)
		m.Ball.LastTeam, m.Ball.LastPlayer, m.originalBallOwner = team, index, team*11+index
		// E06A records the thrower at pickup, before its later throwing action.
		m.originalLastShooter = team*11 + index
		p.originalState, p.originalDirectionHistory, p.AnimationPhase = 10, -1, 0
		p.Facing = OriginalDirection(6)
		if p.Pos.X < 0 {
			p.Facing = OriginalDirection(2)
		}
		m.originalThrowCountdown = 100
		m.originalBallDrawEnabled = false
		return Vec2{}, 0
	}
	var move Vec2
	var step int
	if m.originalRecordHasController(team, index) {
		direction := -1
		if in.Move.Len() > .05 {
			direction = OriginalDirectionIndex(in.Move)
		}
		// DE7E/DEE2/DF46/DFB2 use 1079E before the exclusion and edge
		// guards. It includes the human pace bonus and coordinate clamps.
		move, step = originalHumanRun(p, direction, originalPace(p))
	} else if p.originalState == 7 || p.originalState == 8 {
		if originalDistanceSquared(p.Pos, crossing) > 16 {
			p.Facing = originalTargetHeading(p, crossing)
			move, step = p.Facing, originalPace(p)
		}
	} else {
		move, step = m.originalTacticRun(team, index)
	}
	low, high := int32(3600), int32(5184)
	switch p.originalState {
	case 6, 8:
		low, high = 14400, 20736
	case 7:
		low, high = 7056, 9216
	}
	distance := originalDistanceSquared(p.Pos, crossing)
	if distance < low {
		p.Facing = OriginalDirection((OriginalDirectionIndex(crossing.Sub(Vec2{math.Floor(p.Pos.X), math.Floor(p.Pos.Y)})) + 4) & 7)
		move, step = p.Facing, originalPace(p)
	} else if distance <= high {
		p.originalDirectionHistory = -1
		p.Facing = OriginalDirection(OriginalDirectionIndex(Vec2{math.Floor(m.Ball.Pos.X) - math.Floor(p.Pos.X), math.Floor(m.Ball.Pos.Y) - math.Floor(p.Pos.Y)}))
		move, step = Vec2{}, 0
	}
	// 0xd7d2 changes facing at the edge without changing the selected step.
	if math.Floor(p.Pos.X) < 12 {
		p.Facing = OriginalDirection(2)
	} else if math.Floor(p.Pos.X) >= 888 {
		p.Facing = OriginalDirection(6)
	}
	if step > 0 {
		move = p.Facing
	}
	return move, step
}

func (m *Match) updateOriginalThrower(team, index int, in Input, controlled bool) (Vec2, int) {
	p := &m.Teams[team].Players[index]
	m.originalThrowCountdown--
	if m.originalThrowCountdown > 0 && controlled && in.Move.Len() > .05 {
		direction := OriginalDirectionIndex(in.Move)
		explicit := false
		if m.originalGoalFrame.OutX >= 456 && direction > 0 && direction < 4 {
			direction, explicit = direction+4, true
		} else if m.originalGoalFrame.OutX < 456 && direction > 4 {
			direction, explicit = direction-4, true
		}
		p.Facing = OriginalDirection(direction)
		if in.Fire {
			if !explicit {
				if receiver := m.originalThrowTarget(team, index, &direction); receiver >= 0 {
					m.originalThrowLaunch(team, index, receiver, direction)
				} else {
					m.originalThrowLaunch(team, index, -1, direction)
				}
			} else {
				m.originalThrowLaunch(team, index, -1, direction)
			}
			m.originalThrowCountdown = 0
		}
	} else if m.originalThrowCountdown == 0 {
		direction := -1
		if receiver := m.originalThrowTarget(team, index, &direction); receiver >= 0 {
			m.originalThrowLaunch(team, index, receiver, direction)
		} else {
			m.originalThrowCountdown = 25
		}
	}
	if m.originalThrowCountdown <= 0 && int16(p.AnimationPhase) < 0 {
		p.originalState, p.originalDirectionHistory = 29, -1
		m.originalBallDrawEnabled = true
	}
	step := p.originalStep
	if int16(p.AnimationPhase) >= 0 {
		p.AnimationPhase += uint16(step)
		if p.AnimationPhase>>8 >= 2 {
			p.AnimationPhase = 65535
		}
	}
	// ABBA advances the throwing animation but never moves the actor.
	return Vec2{}, step
}

func (m *Match) originalThrowTarget(team, index int, direction *int) int {
	p := &m.Teams[team].Players[index]
	goal := Vec2{456, 0}
	if m.AttackDirection(team) > 0 {
		goal.Y = 1392
	}
	p.Facing = originalTargetHeading(p, goal)
	preferences := originalGoalPreferences[originalSector(p.Pos, goal)]
	if *direction >= 0 {
		preferences = [8]int{*direction, *direction, *direction, *direction, *direction, *direction, *direction, *direction}
	}
	for _, bearing := range preferences {
		best, distance, opponent := -1, int32(math.MaxInt32), int32(math.MaxInt32)
		for side := range m.Teams {
			for candidate := range m.Teams[side].Players {
				q := &m.Teams[side].Players[candidate]
				if q.originalState == 0 || side == team && candidate == index || q.originalBallDistance > 160000 || OriginalDirectionIndex(Vec2{math.Floor(q.Pos.X) - math.Floor(p.Pos.X), math.Floor(q.Pos.Y) - math.Floor(p.Pos.Y)}) != bearing {
					continue
				}
				if side == team && q.originalBallDistance <= distance {
					best, distance = candidate, q.originalBallDistance
				} else if side != team && q.originalBallDistance <= opponent {
					opponent = q.originalBallDistance
				}
			}
		}
		if best >= 0 && distance >= 1600 && distance <= 90000 && distance < opponent {
			*direction = bearing
			return best
		}
	}
	return -1
}

func (m *Match) originalThrowLaunch(team, index, receiver, direction int) {
	p := &m.Teams[team].Players[index]
	power, lift := 410, int32(50000)
	if receiver >= 0 {
		power = originalSquareRoot(int(m.Teams[team].Players[receiver].originalBallDistance)) + 60
		lift = 30000
		m.originalBallOwner = team*11 + receiver
		q := &m.Teams[team].Players[receiver]
		q.touchCooldown, q.originalOwnerTicks = 0, 100
	}
	components := [8][2]int32{{500, -866}, {707, -707}, {1000, 0}, {707, 707}, {500, 866}, {-707, 707}, {-1000, 0}, {-707, -707}}
	if p.Pos.X > 0 {
		components[0][0], components[4][0] = -500, -500
	}
	p.Facing = OriginalDirection(direction)
	state := originalBallState(m.Ball, m.ballCurve)
	state.VelocityX, state.VelocityY, state.Lift = components[direction][0]*int32(power), components[direction][1]*int32(power), lift
	state.Height = 15<<16 | int32(uint16(state.Height))
	m.Ball.applyOriginalState(state)
	p.originalStep, p.touchCooldown = power>>1, 15
	m.emit("throw", team, index, m.Ball.Pos)
}

func (m *Match) updateOriginalThrowRule() {
	if m.originalRuleState == 3 {
		m.originalRuleWait--
		if m.originalRuleWait >= 0 && m.originalGoalFrame.OutKind != 0 {
			return
		}
		for side := range m.Teams {
			for index := range m.Teams[side].Players {
				p := &m.Teams[side].Players[index]
				if p.originalState != 0 {
					if side*11+index != m.originalThrower {
						p.originalState = 1
					}
					p.originalDirectionHistory = -1
				}
			}
		}
		m.originalRuleState = 0
	} else if m.originalRuleState == 0 && m.originalGoalFrame.OutKind == 1 {
		m.beginOriginalThrowIn()
	}
}
