package engine

import "math"

func originalKeeperHoldingAction(action int) bool {
	switch action {
	case 20, 39, 44, 47, 49, 50:
		return true
	}
	return false
}

// OriginalKeeperPassState translates BFEA..C05E. This held-ball pass has no
// shooting error, uses the receiver's cached ball distance, and draws one lift
// word. The caller supplies the separate height/action/drawing transitions.
func OriginalKeeperPassState(ball, keeper, receiver OriginalBallState, receiverDistance int, word uint16) OriginalBallState {
	distance := originalSquareRoot(receiverDistance)
	if distance == 0 {
		// BE9C only admits cached squared distances1600..90000.
		return ball
	}
	velocity := func(delta int32) int32 {
		scaled := delta >> 4
		quotient := scaled / int32(int16(distance))
		normalized := int16(scaled)
		if quotient >= -32768 && quotient <= 32767 {
			normalized = int16(quotient)
		}
		return (int32(normalized) << 6) + int32(int16(delta>>16))*500
	}
	ball.VelocityX = velocity(receiver.X - keeper.X)
	ball.VelocityY = velocity(receiver.Y - keeper.Y)
	ball.Lift = int32(200+int(word%200)) * 100
	return ball
}

// originalKeeperPassTarget follows BA2C->BB04->BE9C. The final receiver guard
// also visits inactive opponent records; practice explicitly skips that guard.
func (m *Match) originalKeeperPassTarget(team int, p *Player) int {
	goal := Vec2{456, 1392}
	if m.AttackDirection(team) < 0 {
		goal.Y = 0
	}
	p.Facing = originalTargetHeading(p, goal)
	preferences := originalGoalPreferences[originalSector(p.Pos, goal)]
	p.Facing = OriginalDirection(preferences[0])
	var choices [2][8]int
	var distances [2][8]int32
	for side := range choices {
		for direction := range choices[side] {
			choices[side][direction], distances[side][direction] = -1, math.MaxInt32
		}
		actual := team
		if side == 1 {
			actual = 1 - team
		}
		for index := range m.Teams[actual].Players {
			q := &m.Teams[actual].Players[index]
			if q == p || q.originalState == 0 || q.originalBallDistance > 160000 {
				continue
			}
			direction := OriginalDirectionIndex(Vec2{math.Floor(q.Pos.X) - math.Floor(p.Pos.X), math.Floor(q.Pos.Y) - math.Floor(p.Pos.Y)})
			if q.originalBallDistance <= distances[side][direction] {
				choices[side][direction], distances[side][direction] = index, q.originalBallDistance
			}
		}
	}
	for _, direction := range preferences {
		index := choices[0][direction]
		if index < 0 || distances[0][direction] < 1600 || distances[0][direction] > 90000 || distances[0][direction] >= distances[1][direction] {
			continue
		}
		if !m.Config.Practice {
			blocked := false
			for _, opponent := range m.Teams[1-team].Players {
				if originalDistanceSquared(m.Teams[team].Players[index].Pos, opponent.Pos) < 14400 {
					blocked = true
					break
				}
			}
			if blocked {
				continue
			}
		}
		p.Facing = OriginalDirection(direction)
		return index
	}
	return -1
}

func (m *Match) originalKeeperPass(team, receiver int, p *Player) {
	q := &m.Teams[team].Players[receiver]
	position := func(value Vec2) OriginalBallState {
		return OriginalBallState{X: int32(math.Round(value.X * 65536)), Y: int32(math.Round(value.Y * 65536))}
	}
	state := OriginalKeeperPassState(originalBallState(m.Ball, m.ballCurve), position(p.Pos), position(q.Pos), int(q.originalBallDistance), OriginalRandomWord(&m.originalRandomState))
	state.Height = int32(8<<16) | int32(uint16(state.Height))
	m.Ball.applyOriginalState(state)
	m.Ball.TrappedBy, m.originalBallOwner = -1, team*11+receiver
	m.Ball.LastTeam, m.Ball.LastPlayer = team, 0
	m.originalLastShooter = team * 11
	p.touchCooldown, p.lastTouch = 15, m.Tick
	q.touchCooldown, q.originalOwnerTicks = 0, 100
	m.originalResetKeeperReleaseActors()
	p.originalState, p.originalSubstate, p.originalDirectionHistory = 19, 0, -1
	p.AnimationPhase = 0
	// BFEA and1417E..141B4 have no D208 sound call. This event is deliberately
	// distinct from the ordinary audible nine-draw outfield pass.
	m.emit("keeper_pass", team, 0, m.Ball.Pos)
}

func (m *Match) originalResetKeeperReleaseActors() {
	for team := range m.Teams {
		for index := range m.Teams[team].Players {
			p := &m.Teams[team].Players[index]
			if p.originalState != 0 {
				// The B812 team assignment preserves heading history. The
				// keeper caller invalidates only its own history separately.
				p.originalState = 1
			}
		}
	}
}

func (m *Match) originalKeeperRelease(team int, p *Player) {
	// 140DE consumes the decision draw even when no receiver is available.
	if OriginalRandomWord(&m.originalRandomState)%100 >= 20 {
		if receiver := m.originalKeeperPassTarget(team, p); receiver >= 0 && m.originalKeeperRepeatedCatch < 3 {
			m.originalKeeperPass(team, receiver, p)
			return
		}
	}
	m.originalKeeperKick(team, p)
}

func originalKeeperPassAnimation(p *Player) {
	if int16(p.AnimationPhase) >= 0 {
		p.AnimationPhase += 100
		if p.AnimationPhase>>8 >= 3 {
			p.AnimationPhase = 65535
		}
	}
}

func (m *Match) updateOriginalCarryingKeeper(team int, p *Player, dt float64) {
	if p.originalState == 44 {
		m.Ball.Pos = p.Pos
		p.Vel, p.originalStep = Vec2{}, 0
		p.Facing = Vec2{0, float64(m.AttackDirection(team))}
		m.keeperHold--
		if m.keeperHold <= 0 && !m.originalPeriod.PenaltyContest {
			p.originalState, p.originalSubstate, p.originalDirectionHistory = 20, 0, -1
		}
	} else {
		releasing := false
		if p.originalKeeperCarryCycles >= 2 {
			p.originalStep = 0
			m.originalKeeperRelease(team, p)
			releasing = true
		} else if m.Config.Human[team] && m.controls[team].Fire {
			// 140CA is level-sensitive and bypasses the decision draw. The
			// old movement step still reaches AEFC on this clearance update.
			m.originalKeeperKick(team, p)
			releasing = true
		} else {
			m.Ball.Pos = p.Pos
			_, step, arrived := m.originalRunToTarget(p, p.originalKeeperTarget)
			p.originalStep = step
			if arrived {
				m.originalKeeperRelease(team, p)
				releasing = true
			} else {
				p.originalStep = m.originalKeeperCarryStep(team, p)
			}
		}
		p.Vel = Vec2{}
		if releasing && p.originalState == 19 {
			originalKeeperPassAnimation(p)
		} else {
			p.Vel = p.Facing.Mul(float64(p.originalStep) * 1000 * TPS / originalFixedScale)
			p.Pos = p.Pos.Add(p.Vel.Mul(dt))
		}
	}
	p.originalBallDistance = originalDistanceSquared(p.Pos, m.Ball.Pos)
}
