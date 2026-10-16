package engine

// originalScissorStep is the half-phase animation at0xad26. The source retains
// the previous moving step on entry, then its state63 routine clears movement.
func originalScissorStep(p *Player) {
	if int16(p.AnimationPhase) < 0 {
		return
	}
	old := p.AnimationPhase
	p.AnimationPhase += 128
	if old>>8 >= 16 {
		p.AnimationPhase = 65535
	}
}

func (m *Match) updateOriginalSpecialPlayer(team, index int) (Vec2, int) {
	p := &m.Teams[team].Players[index]
	switch p.originalState {
	case 30:
		if p.touchCooldown <= 0 {
			p.originalState, p.originalDirectionHistory = 1, -1
			p.AnimationPhase = 0
		}
		p.Facing = originalTargetHeading(p, m.Ball.Pos)
		return Vec2{}, 0
	case 63:
		if int16(p.AnimationPhase) < 0 {
			p.originalState, p.originalDirectionHistory = 14, -1
			p.AnimationPhase = 0
			p.originalRecoveryLimit = 0x32
			return Vec2{}, 0
		}
		if p.originalBallDistance <= 36 && int(m.Ball.Height) <= 24 && p.touchCooldown <= 0 {
			launch := m.originalShot(p.Facing, p.Attributes.Shooting)
			state := originalBallState(m.Ball, 0)
			state.VelocityX, state.VelocityY, state.Lift = launch.VelocityX, launch.VelocityY, launch.Lift
			m.Ball.applyOriginalState(state)
			m.Ball.TrappedBy, m.originalBallOwner = -1, -1
			m.Ball.LastTeam, m.Ball.LastPlayer = team, index
			m.originalLastShooter = team*11 + index
			p.touchCooldown, p.lastTouch = 15, m.Tick
			m.emit("scissor", team, index, m.Ball.Pos)
		}
		originalScissorStep(p)
		return Vec2{}, 0
	case 14:
		if int16(p.AnimationPhase) < 0 {
			p.originalState, p.originalDirectionHistory = 15, -1
			p.AnimationPhase = 16
			return Vec2{}, 0
		}
		step := p.originalStep - (p.originalStep >> 4)
		if step <= 16 {
			step = 0
		}
		p.AnimationPhase += 32
		if p.AnimationPhase >= 0x300 && p.AnimationPhase > p.originalRecoveryLimit {
			p.AnimationPhase = 65535
		}
		return p.Facing, step
	case 15:
		if int16(p.AnimationPhase) < 0 {
			p.originalState, p.originalDirectionHistory = 1, -1
			p.AnimationPhase = 0
			return Vec2{}, 0
		}
		p.AnimationPhase += 16
		if p.AnimationPhase > 0x500 {
			p.AnimationPhase = 65535
		}
		return Vec2{}, 0
	}
	return Vec2{}, 0
}
