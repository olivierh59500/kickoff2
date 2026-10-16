package engine

import "math"

func originalGoalIntersection(state OriginalBallState, y int) (int, bool) {
	distance := y - int(state.Y>>16)
	if distance < 0 && state.VelocityY>>16 >= 0 || distance >= 0 && state.VelocityY>>16 < 0 {
		return 0, false
	}
	x, ySpeed := int(int16(state.VelocityX>>8)), int(int16(state.VelocityY>>8))
	if ySpeed == 0 {
		return 0, false
	}
	quotient := distance * x / ySpeed
	if quotient < -32768 || quotient > 32767 {
		return 0, false
	}
	return int(state.X>>16) + quotient, true
}

func originalKeeperAnimationStep(p *Player) {
	if int16(p.AnimationPhase) < 0 {
		return
	}
	old := p.AnimationPhase
	p.AnimationPhase += 256
	if old>>8 >= 15 {
		p.AnimationPhase = 65535
	}
}

func originalKeeperDrift(p *Player) {
	p.Pos.X += float64(p.originalKeeperDrift) / originalFixedScale
	p.originalKeeperDrift -= p.originalKeeperDrift >> 4
}

// updateOriginalAnimatedKeeper covers the standing aerial save states 37, 38
// and 39 at 0x1420e..0x142f8, including their horizontal interception drift.
func (m *Match) updateOriginalAnimatedKeeper(team int, p *Player) bool {
	if m.updateOriginalDiveKeeper(team, p) {
		return true
	}
	if p.originalState == 39 {
		originalKeeperDrift(p)
		if int16(p.AnimationPhase) < 0 {
			p.originalSubstate, p.originalState = 0, 44
			p.originalDirectionHistory = -1
			p.AnimationPhase = 0
			m.keeperHold = 50
		} else {
			m.Ball.Pos = p.Pos
			originalKeeperAnimationStep(p)
		}
		p.originalBallDistance = originalDistanceSquared(p.Pos, m.Ball.Pos)
		return true
	}
	if p.originalSubstate == 37 || p.originalSubstate == 38 {
		originalKeeperDrift(p)
		if int16(p.AnimationPhase) >= 0 {
			contact := p.originalBallDistance <= 144 && int(m.Ball.Height) >= 10 && int(m.Ball.Height) <= 32
			if contact {
				// 0x13b7a updates last touch before its cooldown/out gates.
				m.Ball.LastTeam, m.Ball.LastPlayer = team, 0
			}
			if contact && p.touchCooldown <= 0 && !m.originalFoul.Active && m.originalGoalFrame.OutKind == 0 {
				if int(OriginalRandomWord(&m.originalRandomState)%130) < int(math.Round(p.Attributes.Shooting*100)) {
					substate := p.originalSubstate
					facing := p.Facing
					m.originalKeeperCollect(team, p)
					p.originalState, p.originalSubstate = 39, 0
					p.originalDirectionHistory = -1
					p.Facing = facing
					if substate == 37 {
						p.AnimationPhase = 0
					}
				} else {
					m.originalKeeperParry(team, p)
				}
			}
			originalKeeperAnimationStep(p)
		} else if p.originalSubstate == 37 {
			p.originalSubstate = 38
			p.originalDirectionHistory = -1
			p.AnimationPhase = 0
			originalKeeperAnimationStep(p)
		} else {
			p.originalSubstate = 0
			p.originalDirectionHistory = -1
			p.AnimationPhase = 0
		}
		p.originalBallDistance = originalDistanceSquared(p.Pos, m.Ball.Pos)
		return true
	}
	if p.originalState != 1 || p.originalBallDistance >= 9216 {
		return false
	}
	state := originalBallState(m.Ball, m.ballCurve)
	goalY := 0
	if m.AttackDirection(team) < 0 {
		goalY = 1392
	}
	intersection, valid := originalGoalIntersection(state, goalY)
	if !valid || intersection < 396 || intersection > 516 {
		return false
	}
	vx, vy := state.VelocityX>>8, state.VelocityY>>8
	speed := (int64(vx)*int64(vx) + int64(vy)*int64(vy)) >> 16
	if m.originalBallDerivedReady {
		// The original13D98 trigger reads A4-1A02, not velocities
		// that an earlier actor may have changed since integration.
		speed = int64(m.originalBallSpeedSquared >> 16)
	}
	nearLine := m.AttackDirection(team) > 0 && p.Pos.Y < 24 || m.AttackDirection(team) < 0 && p.Pos.Y >= 1368
	if speed < 36 && !(int(m.Ball.Height) > 12 && p.originalBallDistance < 1296 && nearLine) {
		return false
	}
	interception, valid := originalGoalIntersection(state, int(math.Floor(p.Pos.Y)))
	if !valid {
		interception = -1
	}
	difference := clampInt(interception-int(math.Floor(p.Pos.X)), -48, 48)
	drift := int32(difference * 5000)
	estimatedHeight := (state.Height + (state.Lift << 4)) >> 16
	p.originalKeeperDrift = drift
	p.originalState, p.originalStep = 1, 0
	p.originalDirectionHistory = -1
	p.AnimationPhase = 0
	p.Vel = Vec2{}
	if estimatedHeight <= 18 {
		if drift>>16 == 0 {
			p.originalState, p.originalSubstate = 51, 0
			m.keeperHold = 50
			p.Facing = originalTargetHeading(p, m.Ball.Pos)
		} else {
			p.originalSubstate = 45
		}
	} else if drift>>16 != 0 {
		p.originalSubstate = 48
	} else {
		p.originalSubstate = 37
	}
	if p.originalSubstate == 45 || p.originalSubstate == 48 {
		direction := 1
		if drift>>16 < 0 {
			direction = 7
		}
		if m.AttackDirection(team) > 0 {
			direction ^= 2
		}
		p.Facing = OriginalDirection(direction)
		originalKeeperDiveAnimationStep(p)
		p.originalBallDistance = originalDistanceSquared(p.Pos, m.Ball.Pos)
		return true
	}
	if p.originalState == 51 {
		p.originalBallDistance = originalDistanceSquared(p.Pos, m.Ball.Pos)
		return true
	}
	p.Facing = originalTargetHeading(p, m.Ball.Pos)
	originalKeeperAnimationStep(p)
	p.originalBallDistance = originalDistanceSquared(p.Pos, m.Ball.Pos)
	return true
}
