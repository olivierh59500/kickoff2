package engine

func (m *Match) originalFireLatch(team, index int) *bool {
	controller := m.originalPlayerController(team, index)
	return &m.originalFireConsumed[controller]
}

// updateOriginalJump translates state 13 at0xe234 and0x10494. Its movement
// stays on the launch direction; joystick steering only redirects a contact.
func (m *Match) updateOriginalJump(team, index int, in Input, human bool) (Vec2, int) {
	p := &m.Teams[team].Players[index]
	if human {
		p.originalDirectionHistory = -1
	}
	if int16(p.AnimationPhase) < 0 {
		p.jumpTicks, p.ActionTicks = 0, 0
		if !human || !in.Fire {
			p.originalState, p.originalDirectionHistory = 1, -1
			p.AnimationPhase = 0
			if human {
				// 0x104ee clears phase, then still renders through0xac92.
				p.AnimationPhase = 128
			}
		}
		return Vec2{}, 0
	}
	if p.originalBallDistance <= 36 && int(m.Ball.Height) <= 24 && (human || p.touchCooldown <= 0) {
		aim := OriginalDirectionIndex(p.Facing)
		if human {
			if p.touchCooldown <= 0 && in.Move.Len() > .05 {
				p.Facing = OriginalDirection(OriginalDirectionIndex(in.Move))
			}
			aim = OriginalDirectionIndex(p.Facing)
		} else {
			goal := Vec2{456, 40}
			if m.AttackDirection(team) > 0 {
				goal.Y = 1352
			}
			// C1EA uses B8C8 only for the launch direction. It never
			// replaces the jumper's movement heading at player+8.
			aim = OriginalDirectionIndex(goal.Sub(p.Pos))
		}
		state := originalBallState(m.Ball, m.ballCurve)
		launch := OriginalDirectedHeaderState(aim, OriginalDirectionIndex(p.Facing), p.originalStep)
		state.VelocityX, state.VelocityY, state.Lift = launch.VelocityX, launch.VelocityY, launch.Lift
		m.Ball.applyOriginalState(state)
		m.originalBallOwner = -1
		m.Ball.LastTeam, m.Ball.LastPlayer = team, index
		m.originalLastShooter = team*11 + index
		p.touchCooldown, p.lastTouch = 15, m.Tick
		m.emit("header", team, index, m.Ball.Pos)
	}
	old := p.AnimationPhase
	p.JumpHeight = OriginalJumpHeight(44 - 2*clampInt(int(old>>8), 0, 21))
	p.AnimationPhase += 128
	if old>>8 >= 22 {
		p.AnimationPhase = 65535
	}
	return p.Facing, p.originalStep
}

func (m *Match) originalAIJump(team, index int) bool {
	p := &m.Teams[team].Players[index]
	if p.originalState != 1 || int(m.Ball.Height) <= 16 || p.originalBallDistance > 900 || m.originalBallOwner == team*11+index {
		return false
	}
	// Every eligible NPC consumes the original draw at0xdb2e, including misses.
	if OriginalRandomWord(&m.originalRandomState)%10000 > 200 {
		return false
	}
	p.originalState, p.originalDirectionHistory = 13, -1
	// DB48 enters AC92 without clearing the existing running accumulator.
	// The inherited phase can make an NPC's jump much shorter than a human's.
	old := p.AnimationPhase
	p.AnimationPhase += 128
	if old>>8 >= 22 {
		p.AnimationPhase = 65535
	}
	p.jumpTicks = clampInt(44-int(old)/128, 0, 44)
	p.ActionTicks = p.jumpTicks
	p.JumpHeight = OriginalJumpHeight(44 - 2*clampInt(int(old>>8), 0, 21))
	m.emit("jump", team, index, p.Pos)
	return true
}

func (m *Match) originalAINearClaim(team, index int) bool {
	p := &m.Teams[team].Players[index]
	if p.originalState != 1 || p.originalBallDistance > 64 {
		return false
	}
	stop := func() bool {
		// 119CC/11A00 clears the step before rejecting a high ball or
		// an opponent's close possession. B97A still updates the bearing.
		p.originalStep, p.Facing = 0, m.originalChaseHeading(p)
		return true
	}
	owner := m.originalBallOwner
	step := 0
	if owner >= 0 {
		q := &m.Teams[owner/11].Players[owner%11]
		if owner/11 == team {
			if owner%11 == index || originalDistanceSquared(p.Pos, q.Pos) <= 2500 || int(m.Ball.Height) > 12 {
				return false
			}
			q.touchCooldown = 20
			// 0x11a5e does not clear the baseline step before taking
			// a teammate's distant possession pointer.
			step = originalPace(p)
		} else {
			if originalDistanceSquared(p.Pos, q.Pos) <= 576 {
				return false
			}
			if int(m.Ball.Height) > 12 || q.originalBallDistance < 64 {
				return stop()
			}
			q.touchCooldown = 10
		}
	} else if int(m.Ball.Height) > 12 {
		return stop()
	}
	m.originalBallOwner = team*11 + index
	m.Ball.LastTeam, m.Ball.LastPlayer = team, index
	p.originalOwnerTicks, p.lastTouch = 10, m.Tick
	p.originalStep = step
	return true
}
