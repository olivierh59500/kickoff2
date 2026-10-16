package engine

import "math"

// OriginalFreeKickState translates C378's signed fixed-point goal vector,
// perpendicular offset and height cap. The bearing comes from the actor's
// BA42 sector, whereas velocity normalization starts at the ball coordinates.
func OriginalFreeKickState(ball OriginalBallState, actor Vec2, attack, lateral, height int) OriginalBallState {
	goal := Vec2{456, 0}
	if attack > 0 {
		goal.Y = 1392
	}
	dx, dy := (int32(goal.X)<<16)-ball.X, (int32(goal.Y)<<16)-ball.Y
	x, y := int32(int16(dx>>12)), int32(int16(dy>>12))
	root := originalSquareRoot(int(x*x + y*y))
	if root != 0 {
		ball.VelocityX, ball.VelocityY = int32(int16(dx/int32(root)))<<7, int32(int16(dy/int32(root)))<<7
	}
	direction := originalGoalPreferences[originalSector(actor, goal)][0]
	if int16(lateral) >= 0 {
		direction += 2
	} else {
		direction -= 2
		lateral = int(-int16(lateral))
	}
	lateral = min(200, lateral)
	perpendicular := OriginalDirection(direction).Mul(1000)
	ball.VelocityX += int32(perpendicular.X) * int32(lateral)
	ball.VelocityY += int32(perpendicular.Y) * int32(lateral)
	ball.Lift = int32(min(400, int(int16(height)))) * 1000
	return ball
}

func (m *Match) updateOriginalCloseFreeKickParticipant(team, index int) (Vec2, int) {
	p := &m.Teams[team].Players[index]
	move, step := m.originalTacticRun(team, index)
	distance := originalDistanceSquared(p.Pos, m.Ball.Pos)
	if distance < 14400 {
		p.Facing = OriginalDirection(OriginalDirectionIndex(Vec2{math.Floor(m.Ball.Pos.X) - math.Floor(p.Pos.X), math.Floor(m.Ball.Pos.Y) - math.Floor(p.Pos.Y)}) + 4)
		return p.Facing, originalPace(p)
	}
	if distance <= 20736 {
		p.Facing, p.originalDirectionHistory = m.originalBallFacing(p), -1
		return Vec2{}, 0
	}
	return move, step
}

// updateOriginalCloseFreeKickTaker follows105E6..10782. Fire first charges
// height while stationary; release starts a half-pace approach. N/S/E/W inputs
// retain their original distinct shot, pass and retreat branches.
func (m *Match) updateOriginalCloseFreeKickTaker(team, index int, in Input, human bool) (Vec2, int) {
	p, restart := &m.Teams[team].Players[index], &m.originalFoulRestart
	if p.originalState == 56 {
		if !human {
			p.originalState, p.originalDirectionHistory = 31, -1
			return p.Facing, p.originalStep
		}
		m.originalClockRunning = false
		if in.Fire {
			p.originalState, restart.Height, restart.Started = 57, 0, 0
		}
		return Vec2{}, 0
	}
	if p.originalState == 58 {
		if p.touchCooldown >= 2 {
			if in.Move.Len() > .05 {
				direction := OriginalDirectionIndex(in.Move)
				if m.AttackDirection(team) > 0 {
					direction = [...]int{4, 3, 6, 7, 0, 1, 2, 5}[direction]
				}
				restart.Lateral += [...]int16{0, 6, 6, 0, 0, 0, -6, -6}[direction]
				restart.Vertical += [...]int16{4, 4, 0, 0, 0, 0, 0, 4}[direction]
			}
			restart.Heading = int16(m.originalBallHeading)
		} else if m.Ball.LastTeam != team || m.Ball.LastPlayer != index || restart.Lateral == 0 && restart.Vertical == 0 {
			p.originalState, p.originalDirectionHistory = 1, -1
		} else {
			state := OriginalAftertouchImpulse(originalBallState(m.Ball, m.ballCurve), int(restart.Heading), int(restart.Lateral), int(restart.Vertical))
			m.Ball.applyOriginalState(state)
			restart.Lateral -= restart.Lateral >> 5
			if restart.Lateral >= 0 {
				restart.Lateral--
			} else {
				restart.Lateral++
			}
			if restart.Vertical != 0 {
				restart.Vertical--
			}
		}
		return p.Facing, p.originalStep
	}
	if restart.Started == 0 {
		if in.Fire {
			restart.Height += 8
			return p.Facing, p.originalStep
		}
		restart.Started++
		restart.Offset = 0
		m.originalClockRunning = true
	}
	direction := -1
	if in.Move.Len() > .05 {
		direction = OriginalDirectionIndex(in.Move)
	}
	restart.Input = int16(direction)
	if direction >= 0 {
		if m.AttackDirection(team) > 0 {
			direction = [...]int{4, 3, 6, 7, 0, 1, 2, 5}[direction]
		}
		restart.Offset += [...]int16{0, 5, 10, 0, 0, 0, -10, -5}[direction]
	}
	move, _, arrived := m.originalRunToTarget(p, m.Ball.Pos)
	if !arrived {
		return move, originalPace(p) >> 1
	}
	m.Ball.LastTeam, m.Ball.LastPlayer = team, index
	if direction == 0 {
		p.originalState = 59
		p.originalRestartTarget.X = math.Floor(p.Pos.X)
		if m.AttackDirection(team) < 0 {
			p.originalRestartTarget.Y = math.Floor(p.Pos.Y) - 36
		} else {
			// 106DA literally writes+96, not+9A, for a downward retreat.
			p.originalRestartTarget.X = math.Floor(p.Pos.Y) + 36
		}
	} else if direction == 3 || direction == 5 {
		// BB04/BE2E use the nearest teammate in the actual input lane.
		// The two auxiliary actors supply the two backward diagonal lanes.
		target, distance := -1, int32(math.MaxInt32)
		for candidate := range m.Teams[team].Players {
			q := &m.Teams[team].Players[candidate]
			if candidate == index || q.originalState == 0 || q.originalBallDistance > 160000 {
				continue
			}
			bearing := OriginalDirectionIndex(Vec2{math.Floor(q.Pos.X) - math.Floor(p.Pos.X), math.Floor(q.Pos.Y) - math.Floor(p.Pos.Y)})
			if bearing == int(restart.Input) && q.originalBallDistance <= distance {
				target, distance = candidate, q.originalBallDistance
			}
		}
		if target >= 0 {
			m.originalPass(team, index, target)
		}
	} else {
		goal := Vec2{456, 0}
		if m.AttackDirection(team) > 0 {
			goal.Y = 1392
		}
		p.Facing = originalTargetHeading(p, goal)
		state := OriginalFreeKickState(originalBallState(m.Ball, m.ballCurve), p.Pos, m.AttackDirection(team), int(restart.Offset), int(restart.Height))
		m.Ball.applyOriginalState(state)
		m.originalBallOwner, m.Ball.TrappedBy = -1, -1
		p.originalState, p.touchCooldown, p.lastTouch = 58, 15, m.Tick
		restart.Lateral, restart.Vertical = 0, 0
		m.emit("free_kick", team, index, m.Ball.Pos)
	}
	return Vec2{}, 0
}

func (m *Match) updateOriginalFoulPenaltyTaker(team, index int, in Input) (Vec2, int) {
	p, restart := &m.Teams[team].Players[index], &m.originalFoulRestart
	if p.originalState == 41 {
		target := Vec2{math.Floor(m.Ball.Pos.X), math.Floor(m.Ball.Pos.Y) + 24}
		if m.Ball.Pos.Y >= 696 {
			target.Y -= 48
		}
		move, step, _ := m.originalRunToTarget(p, target)
		return move, step
	}
	human := m.Config.Human[team]
	if human && restart.PenaltyHeld == 0 {
		if !in.Fire {
			m.originalAdvanceFoulPenaltyMarker()
			return p.Facing, p.originalStep
		}
		restart.PenaltyHeld = 1
	}
	move, step, arrived := m.originalRunToTarget(p, m.Ball.Pos)
	if human {
		step = 150
		move = p.Facing
	}
	if !arrived {
		if human && in.Fire {
			restart.PenaltyHeld++
		}
		if human {
			m.originalAdvanceFoulPenaltyMarker()
		}
		return move, step
	}
	m.Ball.LastTeam, m.Ball.LastPlayer = team, index
	m.originalLastShooter = team*11 + index
	// B812 changes active action words without11D34's history reset.
	for side := range m.Teams {
		for actor := range m.Teams[side].Players {
			if q := &m.Teams[side].Players[actor]; q.originalState != 0 {
				q.originalState = 1
			}
		}
	}
	m.Teams[1-team].Players[0].originalState = 52
	state := originalBallState(m.Ball, m.ballCurve)
	launch := OriginalPenaltyState(m.Ball.Pos, restart.PenaltyX, int(restart.PenaltyHeld))
	if !human {
		var words [7]uint16
		for i := range words {
			words[i] = OriginalRandomWord(&m.originalRandomState)
		}
		launch = OriginalComputerPenaltyState(m.Ball.Pos, int(math.Round(p.Attributes.Shooting*100)), m.AttackDirection(team), words)
		p.originalState = 30
	}
	state.VelocityX, state.VelocityY, state.Lift = launch.VelocityX, launch.VelocityY, launch.Lift
	m.Ball.applyOriginalState(state)
	m.originalBallOwner, m.Ball.TrappedBy = -1, -1
	p.touchCooldown, p.lastTouch = 15, m.Tick
	restart.PenaltyHeld, restart.PenaltyMarker = 0, false
	m.aftertouchTicks, m.aftertouchLateral, m.aftertouchVertical = 0, 0, 0
	m.emit("penalty", team, index, m.Ball.Pos)
	return move, step
}

func (m *Match) originalAdvanceFoulPenaltyMarker() {
	restart := &m.originalFoulRestart
	if !restart.PenaltyMarker {
		restart.PenaltyMarker = true
		restart.PenaltyX, restart.PenaltyY, restart.PenaltyStep = int(math.Floor(m.Ball.Pos.X)), int(math.Floor(m.Ball.Pos.Y))-50, 3
		if m.Ball.Pos.Y > 696 {
			restart.PenaltyY += 100
		}
	}
	restart.PenaltyDrawX, restart.PenaltyDrawY = restart.PenaltyX, restart.PenaltyY
	restart.PenaltyX, restart.PenaltyStep = OriginalPenaltyAimStep(restart.PenaltyX, restart.PenaltyStep)
}

// updateOriginalFoulPenaltyKeeper follows142FA and1458E. Human Fire starts
// one of the source low/high dives; the CPU chooses among four lateral inputs.
func (m *Match) updateOriginalFoulPenaltyKeeper(team int, p *Player, dt float64) bool {
	if p.originalState == 43 {
		target := Vec2{456, 2}
		if m.AttackDirection(team) < 0 {
			target.Y = 1390
		}
		move, step, arrived := m.originalRunToTarget(p, target)
		if arrived {
			p.AnimationPhase = 0
			if m.Config.Human[team] && m.boundPrimaryPlayer(team) < 0 {
				m.Teams[team].selectOwnPlayer(0)
			}
		}
		p.originalStep = step
		p.Vel = move.Mul(float64(step) * 1000 * TPS / originalFixedScale)
		p.Pos = p.Pos.Add(p.Vel.Mul(dt))
		p.originalBallDistance = originalDistanceSquared(p.Pos, m.Ball.Pos)
		return true
	}
	if p.originalState != 52 {
		return false
	}
	p.originalStep, p.Vel = 0, Vec2{}
	direction := -1
	if m.Config.Human[team] {
		if !m.controls[team].Fire {
			p.originalBallDistance = originalDistanceSquared(p.Pos, m.Ball.Pos)
			return true
		}
		if m.controls[team].Move.Len() > .05 {
			direction = OriginalDirectionIndex(m.controls[team].Move)
		}
	} else {
		direction = 1 + int(OriginalRandomWord(&m.originalRandomState)%4)
		if direction >= 3 {
			direction += 3
		}
	}
	high := direction >= 0 && (direction+1)&7 < 3
	drift := int32(0)
	if direction >= 0 && (!high || direction != 0) {
		drift = 160000
		if direction&4 != 0 {
			drift = -drift
		}
	}
	p.originalKeeperDrift = drift
	p.originalState, p.originalSubstate, p.originalDirectionHistory, p.AnimationPhase = 1, 0, -1, 0
	if drift == 0 {
		// The centered branches call B82A, which also retains its sixteen-sector
		// target history. B91A would produce the same facing but lose that word.
		p.Facing = originalTargetHeading(p, m.Ball.Pos)
		if high {
			p.originalSubstate = 37
			originalKeeperAnimationStep(p)
		} else {
			p.originalState, m.keeperHold = 51, 100
		}
	} else {
		direction = 1
		if drift < 0 {
			direction = 7
		}
		if m.AttackDirection(team) > 0 {
			direction ^= 2
		}
		p.Facing, p.originalSubstate = OriginalDirection(direction), 45
		if high {
			p.originalSubstate = 48
		}
		originalKeeperDiveAnimationStep(p)
	}
	p.originalBallDistance = originalDistanceSquared(p.Pos, m.Ball.Pos)
	return true
}
