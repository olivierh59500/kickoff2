package engine

import "math"

func (m *Match) kick(team, index int, direction Vec2, power, lift float64, kind string) {
	p := &m.Teams[team].Players[index]
	if direction.Len() < 0.1 {
		direction = p.Facing
	}
	direction = direction.Unit()
	accuracy := p.Attributes.Shooting
	if kind == "pass" {
		accuracy = p.Attributes.Passing
	}
	if kind != "shot" {
		direction = rotate(direction, (m.random()-0.5)*(1-accuracy)*0.15)
	}
	m.Ball.TrappedBy = -1
	m.originalBallOwner = -1
	// The original shot retains any remaining corner curve and the keeper's
	// independent countdown while replacing the launch velocity.
	m.Ball.Vel = direction.Mul(power)
	m.Ball.Height = math.Max(0, m.Ball.Height)
	m.Ball.VerticalSpeed = lift
	m.Ball.LastTeam = team
	m.Ball.LastPlayer = index
	m.originalLastShooter = team*11 + index
	p.lastTouch = m.Tick
	p.touchCooldown = 15
	if kind == "shot" || kind == "free_kick" || kind == "penalty" {
		m.aftertouchTeam = team
		m.aftertouchPlayer = index
		m.aftertouchFreeKick = kind == "free_kick"
		m.aftertouchTicks = 15
		m.aftertouchLateral, m.aftertouchVertical = 0, 0
		m.aftertouchHeading = OriginalDirectionIndex(direction)
		m.aftertouchNewShot = true
	} else {
		m.aftertouchTicks = 0
		m.aftertouchLateral, m.aftertouchVertical = 0, 0
	}
	if kind == "shot" {
		// Source 0x10266 clears the launch-frame step; the state transition
		// at 0x105d6 leaves the following frame stationary as well.
		p.shotPauseTicks = 2
		p.originalState = 44
		state := m.originalShot(direction, p.Attributes.Shooting)
		m.Ball.Vel = Vec2{float64(state.VelocityX) * TPS / originalFixedScale, float64(state.VelocityY) * TPS / originalFixedScale}
		m.Ball.VerticalSpeed = float64(state.Lift) * TPS / originalFixedScale
	}
	m.emit(kind, team, index, m.Ball.Pos)
}

func (m *Match) updateAftertouch(input [2]Input) {
	if m.aftertouchTicks <= 0 && m.aftertouchLateral == 0 && m.aftertouchVertical == 0 {
		return
	}
	if !m.Config.Aftertouch || m.aftertouchTeam < 0 {
		m.aftertouchTicks = 0
		m.aftertouchLateral, m.aftertouchVertical = 0, 0
		return
	}
	if m.aftertouchNewShot {
		m.aftertouchNewShot = false
		return
	}
	if !m.aftertouchFreeKick {
		// Ordinary shot input is applied in its actor, before control can
		// change and before later actors interact with the shared ball.
		return
	}
	if m.aftertouchTicks >= 2 {
		move := m.aftertouchInput(input)
		if move.Len() > 0.1 {
			direction := OriginalDirectionIndex(move)
			// 0x10706 reflects joystick directions for a downward attack.
			if m.AttackDirection(m.aftertouchTeam) > 0 {
				direction = [...]int{4, 3, 6, 7, 0, 1, 2, 5}[direction]
			}
			m.aftertouchLateral += [...]int{0, 6, 6, 0, 0, 0, -6, -6}[direction]
			m.aftertouchVertical += [...]int{4, 4, 0, 0, 0, 0, 0, 4}[direction]
		}
		m.aftertouchHeading = OriginalDirectionIndex(m.Ball.Vel)
		m.aftertouchTicks--
		return
	}
	m.aftertouchTicks = 0
	state := OriginalAftertouchImpulse(originalBallState(m.Ball, m.ballCurve), m.aftertouchHeading, m.aftertouchLateral, m.aftertouchVertical)
	m.Ball.applyOriginalState(state)
	m.aftertouchLateral -= m.aftertouchLateral >> 5
	if m.aftertouchLateral >= 0 {
		m.aftertouchLateral--
	} else {
		m.aftertouchLateral++
	}
	if m.aftertouchVertical > 0 {
		m.aftertouchVertical--
	}
}

func (m *Match) updateBall() {
	m.ballPrevious = m.Ball.Pos
	m.ballPreviousHeight = m.Ball.Height
	// Held-ball positions are copied by the holder's action routine after this
	// integration step. Copying them here changes other players' cached distance.
	state := originalBallState(m.Ball, m.ballCurve)
	var bounce bool
	before := state
	state, bounce = OriginalBallStep(state, m.Config.Pitch, m.originalWindX, m.originalWindY)
	m.originalGoalFrame.EndsSwapped = m.AttackDirection(0) > 0
	var frameSound bool
	state, m.originalGoalFrame, frameSound = OriginalGoalFrameStep(state, before.Y, m.originalGoalFrame)
	if m.originalBallDrawEnabled {
		state, m.originalAdvertising = OriginalAdvertisingStep(state, m.originalAdvertising)
	}
	if frameSound {
		m.emit("post", m.Ball.LastTeam, m.Ball.LastPlayer, Vec2{float64(state.X) / originalFixedScale, float64(state.Y) / originalFixedScale})
	}
	if bounce {
		m.emit("bounce", m.Ball.LastTeam, m.Ball.LastPlayer, Vec2{float64(state.X) / originalFixedScale, float64(state.Y) / originalFixedScale})
		m.Events[len(m.Events)-1].SoundSample = OriginalBounceSample(before)
	}
	m.Ball.applyOriginalState(state)
	m.ballCurve = state.Curve
	// A89C caches this word before actor actions can launch another shot.
	// Keeper decisions later in the actor loop consume that saved value.
	vx, vy := int32(int16(state.VelocityX>>8)), int32(int16(state.VelocityY>>8))
	m.originalBallSpeedSquared = vx*vx + vy*vy
	m.originalBallGoalGuide = originalGoalGuide(state)
	m.originalBallHeading = OriginalDirectionIndex(Vec2{float64(state.VelocityX >> 16), float64(state.VelocityY >> 16)})
	m.originalBallDerivedReady = true
}

func (m *Match) checkBoundaries() {
	// 0x125fa dispatches the current rule; only live rule0 enters12036.
	if m.originalRuleState != 0 {
		return
	}
	// 12026 gives a pending referee foul priority over boundary dispatch.
	if m.originalFoul.Active {
		return
	}
	if m.originalGoalFrame.OutKind >= 3 {
		m.scoreGoal(int(m.originalGoalFrame.OutKind) - 3)
		return
	}
	if m.originalGoalFrame.OutKind == 2 {
		top := m.originalGoalFrame.OutY < 0
		defender := m.DefendingTeam(top)
		if m.originalPendingSubstitution.Word > 0 && m.originalPendingSubstitution.Word <= 2 {
			m.originalRuleState = 24
			if m.Config.Practice || m.Ball.LastTeam == defender {
				m.originalRuleState = 25
			}
			if m.originalBeginPendingSubstitution() {
				return
			}
		}
		if m.Config.Practice || m.Ball.LastTeam == defender {
			team := 1 - defender
			if m.Config.Practice {
				team = 0
			}
			m.beginOriginalCorner(team)
		} else {
			m.beginOriginalGoalKick(defender)
		}
		return
	}
	if m.originalGoalFrame.OutKind == 1 && m.originalPendingSubstitution.Word > 0 && m.originalPendingSubstitution.Word <= 2 {
		m.originalRuleState = 21
		m.originalBeginPendingSubstitution()
	}
}

func (m *Match) scoreGoal(team int) {
	if m.Config.Practice && team == 0 {
		m.beginOriginalPracticeGoal()
		return
	}
	m.beginOriginalMatchGoal(team)
}
