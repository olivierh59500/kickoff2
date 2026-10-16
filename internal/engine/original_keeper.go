package engine

import "math"

// originalKeeperTarget follows the ordinary positioning and low-ball pursuit
// branches at 0x13d00 and 0x13dd8. Dive states have separate source behavior.
func (m *Match) originalKeeperTarget(team int, p *Player) Vec2 {
	x := math.Floor(m.Ball.Pos.X)
	if x < 360 || x > 552 {
		x = 456
	} else {
		x = clamp(x, 426, 486)
	}
	y := 6.0
	if m.AttackDirection(team) < 0 {
		y = PitchHeight - 6
	}
	target := Vec2{x, y}
	if p.originalBallDistance > 28900 {
		return target
	}
	state := originalBallState(m.Ball, m.ballCurve)
	vx, vy := state.VelocityX>>8, state.VelocityY>>8
	speed := int64(vx)*int64(vx) + int64(vy)*int64(vy)
	if m.originalBallDerivedReady {
		speed = int64(m.originalBallSpeedSquared)
	}
	if int(m.Ball.Height) > 12 && speed>>16 > 4 {
		// Project an incoming high ball to the six-unit keeper line.
		if intersection, valid := originalGoalIntersection(state, int(y)); valid && intersection >= 408 && intersection <= 504 {
			return Vec2{float64(intersection), y}
		}
		return target
	}
	// 13DBC falls through to the area pursuit at13DD8 for a slow high
	// ball as well. Height alone does not force the keeper to its goal line.
	ballY := math.Floor(m.Ball.Pos.Y)
	inside := m.Ball.Pos.X >= 192 && m.Ball.Pos.X <= 720
	if m.AttackDirection(team) > 0 {
		inside = inside && ballY >= 0 && ballY <= 216
	} else {
		inside = inside && ballY >= 1176 && ballY <= 1392
	}
	if !inside {
		return target
	}
	// 13DE6/13E20 consume A89C's integration-time heading word. Its
	// signed integer velocity components can select another octant than the
	// full fractional displacement, especially after a ground bounce.
	bearing := OriginalDirectionIndex(Vec2{float64(state.VelocityX >> 16), float64(state.VelocityY >> 16)})
	if m.originalBallDerivedReady {
		bearing = m.originalBallHeading
	}
	if speed>>16 >= 3 {
		if m.AttackDirection(team) > 0 && (bearing == 3 || bearing == 4 || bearing == 5) {
			return target
		}
		if m.AttackDirection(team) < 0 && (bearing == 7 || bearing == 0 || bearing == 1) {
			return target
		}
	}
	return Vec2{math.Floor(m.Ball.Pos.X), ballY}
}

func (m *Match) originalKeeperCatch(team int, p *Player) bool {
	if p.touchCooldown > 0 || p.originalBallDistance > 144 || int(m.Ball.Height) > 12 {
		return false
	}
	// 0x13e90 records the approaching keeper as last toucher before
	// 0x13e94 rejects a ball already declared out or scored.
	// 13E80 updates the repeated-contact word even if a foul later prevents
	// collection. Release decisions consume this same signed source word.
	if m.Ball.LastTeam == team && m.Ball.LastPlayer == 0 {
		m.originalKeeperRepeatedCatch = int(int16(m.originalKeeperRepeatedCatch + 1))
	} else {
		m.originalKeeperRepeatedCatch = 0
	}
	m.Ball.LastTeam, m.Ball.LastPlayer = team, 0
	if m.originalGoalFrame.OutKind != 0 || m.originalFoul.Active {
		return false
	}
	// The normal approach at 0x13e90 directly calls 0x13c42. The stochastic
	// routine 0x13b7a belongs to dive/animated saves, not this normal catch.
	m.originalKeeperCollect(team, p)
	p.Facing = originalTargetHeading(p, m.Ball.Pos)
	return true
}

func (m *Match) originalKeeperCollect(team int, p *Player) {
	m.originalBallDrawEnabled = false
	state := originalBallState(m.Ball, 0)
	state.X = int32(math.Floor(p.Pos.X))<<16 | int32(uint16(state.X))
	state.Y = int32(math.Floor(p.Pos.Y))<<16 | int32(uint16(state.Y))
	state.Height = int32(uint16(state.Height))
	state.VelocityX, state.VelocityY, state.Lift = 0, 0, 0
	m.Ball.applyOriginalState(state)
	m.Ball.TrappedBy = team * 11
	m.Ball.LastTeam, m.Ball.LastPlayer = team, 0
	m.originalBallOwner = team * 11
	for side := range m.Teams {
		for index := range m.Teams[side].Players {
			q := &m.Teams[side].Players[index]
			// B812 leaves inactive records untouched, including injured players
			// removed without a red card when no replacement is available.
			if q.originalState == 0 || q.SentOff || m.Config.Practice && side == 1 && index != 0 {
				continue
			}
			q.originalState = 17
			if side == team {
				q.originalState = 16
			}
		}
	}
	p.originalState, p.originalStep, p.lastTouch = 44, 0, m.Tick
	m.keeperHold = 50
	p.originalKeeperCarryCycles = 0
	// 0x13c74 chooses one of three release positions after catching.
	p.originalKeeperTarget = Vec2{float64(276 + 180*int(OriginalRandomWord(&m.originalRandomState)%3)), 190}
	if m.AttackDirection(team) < 0 {
		p.originalKeeperTarget.Y = 1202
	}
	m.emit("save", team, 0, m.Ball.Pos)
}

func sourceShiftIntegerWord(value int32) int32 {
	return int32(int16(value>>16)>>1)<<16 | int32(uint16(value))
}

func (m *Match) originalKeeperParry(team int, p *Player) {
	state := originalBallState(m.Ball, m.ballCurve)
	sample := 10
	if int(OriginalRandomWord(&m.originalRandomState)%100) <= int(math.Round(p.Attributes.Passing*100)) {
		sample = 8
		state.VelocityY = sourceShiftIntegerWord(-state.VelocityY)
	}
	// 13BCC/13BE2 request the original 19DC8/19DCC program before
	// the remaining parry variation draws. Successful collection is silent.
	m.emit("keeper_parry", team, 0, p.Pos)
	m.Events[len(m.Events)-1].SoundSample = sample
	speed := state.VelocityY
	if speed < 0 {
		speed = -speed
	}
	speed = int32(int16(speed >> 8))
	if speed > 0 {
		variation := int32(OriginalRandomWord(&m.originalRandomState)%uint16(speed)) - (speed >> 1)
		state.VelocityX += variation * 256
		state.VelocityY = sourceShiftIntegerWord(state.VelocityY)
		state.Lift += int32(OriginalRandomWord(&m.originalRandomState)%1000) * 100
		p.touchCooldown = 15
	}
	m.Ball.applyOriginalState(state)
}

func (m *Match) updateOriginalKeeper(team int, p *Player, dt float64) {
	if p.originalState == 35 || p.originalState == 36 {
		// The keeper dispatch uses139F0 rather than the outfield celebration.
		p.originalState, p.originalSubstate, p.originalDirectionHistory = 1, 0, -1
		p.originalBallDistance = originalDistanceSquared(p.Pos, m.Ball.Pos)
		return
	}
	// A4-1152 and A4-1844 are shared keeper globals. Preserve their value
	// across the two team dispatches, including AEFC's animation wraps.
	p.originalKeeperDrift = m.originalKeeperGlobalDrift
	p.originalKeeperCarryCycles = m.originalKeeperAnimationCycles
	defer func() {
		if p.originalSubstate == 0 && p.originalState != 39 {
			advanceOriginalAnimation(p)
		}
		// Keeper handlers also pass their final sprite origin through EEA0.
		// Retaining the entrance flag makes an ordinary keeper invisible after
		// scrolling back into view, even though its position remains correct.
		p.originalVisible = m.originalPlayerVisible(p)
		m.originalKeeperGlobalDrift = p.originalKeeperDrift
		m.originalKeeperAnimationCycles = p.originalKeeperCarryCycles
	}()
	if !p.originalDistanceReady {
		p.originalBallDistance = originalDistanceSquared(p.Pos, m.Ball.Pos)
		p.originalDistanceReady = true
	}
	// 138CA dispatches a nonzero subaction before the ordinary action word.
	// Period/penalty freezes54 therefore cannot erase a dive already running.
	if p.originalSubstate != 0 && m.updateOriginalAnimatedKeeper(team, p) {
		return
	}
	if p.originalState == 33 {
		move, step := m.updateOriginalEnteringPlayer(team, p)
		p.originalStep = step
		p.Vel = move.Mul(float64(step) * 1000 * TPS / originalFixedScale)
		p.Pos = p.Pos.Add(p.Vel.Mul(dt))
		p.originalBallDistance = originalDistanceSquared(p.Pos, m.Ball.Pos)
		p.originalVisible = m.originalPlayerVisible(p)
		return
	}
	if m.updateOriginalFoulPenaltyKeeper(team, p, dt) {
		return
	}
	if p.originalState == 4 {
		// 138F4 maps action 4 to 13EF2: the keeper leaves through the same
		// (-400,680/712) target as an outfield actor at the end of a period.
		move, step := m.updateOriginalDepartingPlayer(team, p)
		p.originalStep = step
		p.Vel = move.Mul(float64(step) * 1000 * TPS / originalFixedScale)
		p.Pos = p.Pos.Add(p.Vel.Mul(dt))
		p.originalBallDistance = originalDistanceSquared(p.Pos, m.Ball.Pos)
		return
	}
	if p.originalState == 54 || p.originalState == 55 {
		move, step := Vec2{}, 0
		if p.originalState == 55 {
			move, step, _ = m.originalRunToTarget(p, p.originalRestartTarget)
		}
		p.originalStep = step
		p.Vel = move.Mul(float64(step) * 1000 * TPS / originalFixedScale)
		p.Pos = p.Pos.Add(p.Vel.Mul(dt))
		p.originalBallDistance = originalDistanceSquared(p.Pos, m.Ball.Pos)
		return
	}
	if m.updateOriginalAnimatedKeeper(team, p) {
		return
	}
	if m.updateOriginalGoalKickKeeper(team, p, dt) {
		return
	}
	if p.originalState == 19 {
		p.Vel = Vec2{}
		if int16(p.AnimationPhase) < 0 {
			p.originalState, p.originalSubstate, p.originalDirectionHistory = 1, 0, -1
			p.AnimationPhase = 0
			m.originalBallDrawEnabled = true
			p.Vel = p.Facing.Mul(float64(p.originalStep) * 1000 * TPS / originalFixedScale)
			p.Pos = p.Pos.Add(p.Vel.Mul(dt))
		} else {
			originalKeeperPassAnimation(p)
		}
		p.originalBallDistance = originalDistanceSquared(p.Pos, m.Ball.Pos)
		return
	}
	if p.originalState == 61 || p.originalState == 62 {
		move, step := m.updateOriginalCornerActor(team, 0, Input{})
		p.originalStep = step
		p.Vel = move.Mul(float64(step) * 1000 * TPS / originalFixedScale)
		p.Pos = p.Pos.Add(p.Vel.Mul(dt))
		p.originalBallDistance = originalDistanceSquared(p.Pos, m.Ball.Pos)
		return
	}
	if p.originalState == 5 || p.originalState == 6 || p.originalState == 21 || p.originalState == 22 || p.originalState == 26 || p.originalState == 27 || p.originalState == 60 {
		move, step := m.originalTacticRun(team, 0)
		p.originalStep = step
		p.Vel = move.Mul(float64(step) * 1000 * TPS / originalFixedScale)
		p.Pos = p.Pos.Add(p.Vel.Mul(dt))
		p.originalBallDistance = originalDistanceSquared(p.Pos, m.Ball.Pos)
		return
	}
	if p.originalState == 17 {
		p.Vel, p.originalStep = Vec2{}, 0
		p.originalBallDistance = originalDistanceSquared(p.Pos, m.Ball.Pos)
		return
	}
	if p.originalState == 2 || p.originalState == 3 {
		target, ok := m.restartTacticPosition(team, 0, KickOff, p.originalState == 2, m.Ball.Pos, m.AttackDirection(team))
		if ok && originalDistanceSquared(p.Pos, target) > 16 {
			p.Facing = originalTargetHeading(p, target)
			p.originalStep = originalPace(p)
			p.Vel = p.Facing.Mul(float64(p.originalStep) * 1000 * TPS / originalFixedScale)
			p.Pos = p.Pos.Add(p.Vel.Mul(dt))
		} else {
			p.Vel, p.originalStep = Vec2{}, 0
			p.originalDirectionHistory = -1
			p.Facing = OriginalDirection(OriginalDirectionIndex(m.Ball.Pos.Sub(p.Pos)))
		}
		p.originalBallDistance = originalDistanceSquared(p.Pos, m.Ball.Pos)
		return
	}
	if m.Ball.TrappedBy == team*11 {
		m.updateOriginalCarryingKeeper(team, p, dt)
		return
	}
	target := m.originalKeeperTarget(team, p)
	pursuing := target == (Vec2{math.Floor(m.Ball.Pos.X), math.Floor(m.Ball.Pos.Y)})
	if pursuing && p.originalBallDistance <= 144 {
		// 13E64 clears the step before the height/cooldown/out checks.
		// A nearby high ball therefore stops the keeper without a catch.
		if !m.originalKeeperCatch(team, p) {
			p.Facing = originalTargetHeading(p, m.Ball.Pos)
		}
		p.Vel, p.originalStep = Vec2{}, 0
		p.originalBallDistance = originalDistanceSquared(p.Pos, m.Ball.Pos)
		return
	}
	if pursuing || originalDistanceSquared(p.Pos, target) > 16 {
		if pursuing {
			p.Facing = m.originalChaseHeading(p)
		} else {
			p.Facing = originalTargetHeading(p, target)
		}
		p.originalStep = originalPace(p)
		p.Vel = p.Facing.Mul(float64(p.originalStep) * 1000 * TPS / originalFixedScale)
		p.Pos = p.Pos.Add(p.Vel.Mul(dt))
	} else {
		p.Vel = Vec2{}
		p.originalStep = 0
		p.originalDirectionHistory = -1
		p.Facing = OriginalDirection(OriginalDirectionIndex(Vec2{math.Floor(m.Ball.Pos.X) - math.Floor(p.Pos.X), math.Floor(m.Ball.Pos.Y) - math.Floor(p.Pos.Y)}))
	}
	p.originalBallDistance = originalDistanceSquared(p.Pos, m.Ball.Pos)
}

// originalKeeperKick translates the clearance at 0xc060. The source aims toward
// the opposite end, makes power proportional to goal distance, and adds one
// random remainder below one hundred. Carry-state pass choices remain separate.
func (m *Match) originalKeeperKick(team int, p *Player) {
	if !p.Keeper {
		// DDEC calls C060 and records its shooter before the same AAB6
		// movement. It does not perform the keeper's action/phase/team reset.
		m.originalKeeperClearance(team, p)
		for index := range m.Teams[team].Players {
			if p == &m.Teams[team].Players[index] {
				m.originalLastShooter = team*11 + index
				m.Ball.LastTeam, m.Ball.LastPlayer, p.lastTouch = team, index, m.Tick
				m.emit("keeper_kick", team, index, m.Ball.Pos)
				break
			}
		}
		return
	}
	m.originalBallDrawEnabled = true
	m.originalKeeperClearance(team, p)
	m.originalAdjustHumanKeeperClearance(team)
	m.Ball.LastTeam, m.Ball.LastPlayer = team, 0
	p.originalState, p.lastTouch = 1, m.Tick
	p.originalDirectionHistory = -1
	p.AnimationPhase = 0xd00
	m.originalResetKeeperReleaseActors()
	m.emit("keeper_kick", team, 0, m.Ball.Pos)
}

// originalKeeperClearance is C060 itself. Its callers establish their own
// action and rule transitions; the numeric launch does not reset other actors.
func (m *Match) originalKeeperClearance(team int, p *Player) {
	goal := Vec2{456, 1352}
	if m.AttackDirection(team) < 0 {
		goal.Y = 40
	}
	power := (originalSquareRoot(int(originalDistanceSquared(p.Pos, goal))) >> 2) + 60 + int(OriginalRandomWord(&m.originalRandomState)%100)
	direction := OriginalDirectionIndex(Vec2{goal.X - math.Floor(p.Pos.X), goal.Y - math.Floor(p.Pos.Y)})
	state := originalBallState(m.Ball, m.ballCurve)
	delta := OriginalDirection(direction).Mul(float64(power) * 1000)
	state.VelocityX, state.VelocityY, state.Lift = int32(math.Round(delta.X)), int32(math.Round(delta.Y)), int32(power*700)
	m.Ball.applyOriginalState(state)
	m.Ball.TrappedBy, m.originalBallOwner = -1, -1
	p.Facing = OriginalDirection(direction)
	p.touchCooldown = 15
}
