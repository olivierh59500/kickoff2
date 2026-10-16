package engine

import "math"

// E2D4/E2F2 let fixed participants move with 1079E, then E30E forces
// them out of either penalty area at their ordinary pace, including at rest.
func (m *Match) updateOriginalFixedGoalKickActor(team, index int, input Input) (Vec2, int) {
	p := &m.Teams[team].Players[index]
	direction := -1
	if input.Move.Len() > .05 {
		direction = OriginalDirectionIndex(input.Move)
	}
	move, step := originalHumanRun(p, direction, originalPace(p))
	x, y := math.Floor(p.Pos.X), math.Floor(p.Pos.Y)
	if x >= 192 && x <= 720 && (y < 216 || y >= 1176) {
		direction = 4
		if y >= 1176 {
			direction = 0
		}
		p.Facing = OriginalDirection(direction)
		return p.Facing, originalPace(p)
	}
	return move, step
}

// beginOriginalGoalKick translates12484. Both teams walk the goal-kick tactic
// targets while the ball continues its free integration outside the end line.
func (m *Match) beginOriginalGoalKick(team int) {
	for side := range m.Teams {
		for index := range m.Teams[side].Players {
			p := &m.Teams[side].Players[index]
			if p.originalState == 0 {
				continue
			}
			p.originalState, p.originalDirectionHistory = 17, -1
			if side == team {
				p.originalState = 16
			}
		}
	}
	m.originalRuleState = 5
	m.Ball.LastTeam, m.Ball.LastPlayer = -1, -1
	m.Restart = Restart{Kind: GoalKick, Team: team, Player: 0}
	m.notice("GOAL KICK", 350)
	m.emit("whistle", team, 0, m.Ball.Pos)
}

// updateOriginalGoalKickKeeper follows13F22..14086. State53's first wait only
// counts after reaching the preparation spot; state18 then waits before its
// full-pace run-up and the separate C060 clearance caller.
func (m *Match) updateOriginalGoalKickKeeper(team int, p *Player, dt float64) bool {
	if p.originalState != 16 && p.originalState != 53 && p.originalState != 18 {
		return false
	}
	if p.originalState == 16 {
		m.keeperHold = 100
		p.originalState, p.originalSubstate, p.originalDirectionHistory = 53, 0, -1
	}
	move, step := Vec2{}, p.originalStep
	if p.originalState == 53 {
		target := Vec2{348, 12}
		if m.originalGoalFrame.OutX >= 456 {
			target.X = 564
		}
		if m.AttackDirection(team) < 0 {
			target.Y = 1380
		}
		var arrived bool
		move, step, arrived = m.originalRunToTarget(p, target)
		if arrived {
			m.keeperHold--
			if m.keeperHold <= 0 {
				state := originalBallState(m.Ball, m.ballCurve)
				y := int32(66)
				if m.AttackDirection(team) < 0 {
					y = 1326
				}
				// MOVE.W updates the integer coordinate without replacing
				// its fractional word, unlike a native position teleport.
				state.X = int32(target.X)<<16 | int32(uint16(state.X))
				state.Y = y<<16 | int32(uint16(state.Y))
				state.Height, state.VelocityX, state.VelocityY, state.Lift = 0, 0, 0, 0
				m.Ball.applyOriginalState(state)
				m.originalBallDrawEnabled = true
				p.originalState, p.originalSubstate, p.originalDirectionHistory = 18, 0, -1
				m.keeperHold = max(25, int(OriginalRandomWord(&m.originalRandomState)%150)+100-step)
				p.AnimationPhase = 0
				p.Facing = originalTargetHeading(p, m.Ball.Pos)
				move, step = Vec2{}, 0
			}
		}
		if step != 0 {
			step = m.originalKeeperCarryStep(team, p)
		}
	} else {
		// TST.W is level-sensitive here, including an already held button.
		if !m.Config.Human[team] || !m.controls[team].Fire {
			m.keeperHold--
			if m.keeperHold >= 0 {
				p.Vel = p.Facing.Mul(float64(step) * 1000 * TPS / originalFixedScale)
				p.Pos = p.Pos.Add(p.Vel.Mul(dt))
				p.originalBallDistance = originalDistanceSquared(p.Pos, m.Ball.Pos)
				return true
			}
		}
		m.keeperHold = -1
		var arrived bool
		move, step, arrived = m.originalRunToTarget(p, m.Ball.Pos)
		if arrived {
			m.Ball.LastTeam, m.Ball.LastPlayer = team, 0
			m.originalLastShooter = team * 11
			m.originalKeeperClearance(team, p)
			m.originalAdjustHumanKeeperClearance(team)
			p.originalState, p.originalSubstate, p.originalDirectionHistory = 1, 0, -1
			p.AnimationPhase, p.lastTouch = 0xd00, m.Tick
			m.emit("keeper_kick", team, 0, m.Ball.Pos)
		}
	}
	p.originalStep = step
	p.Vel = move.Mul(float64(step) * 1000 * TPS / originalFixedScale)
	p.Pos = p.Pos.Add(p.Vel.Mul(dt))
	p.originalBallDistance = originalDistanceSquared(p.Pos, m.Ball.Pos)
	return true
}

// These nine signed-word pairs are the neutral/N/NE/E/SE/S/SW/W/NW
// tables at A4-4D9C and A4-4D78, respectively.
var originalGoalKickUpAdjust = [9][2]int16{{0, 0}, {0, -2}, {2, -2}, {2, 0}, {2, 1}, {0, 1}, {-2, 1}, {-2, 0}, {-2, -2}}
var originalGoalKickDownAdjust = [9][2]int16{{0, 0}, {0, -1}, {2, -1}, {2, 0}, {2, 2}, {0, 2}, {-2, 2}, {-2, 0}, {-2, -1}}

// 14042 and14116 apply the same live joystick table after goal-kicks and
// held-ball clearances. ADD.W modifies each velocity's integer word only.
func (m *Match) originalAdjustHumanKeeperClearance(team int) {
	if !m.Config.Human[team] {
		return
	}
	state := originalBallState(m.Ball, m.ballCurve)
	choice := 0
	if m.controls[team].Move.Len() > .05 {
		choice = OriginalDirectionIndex(m.controls[team].Move) + 1
	}
	table := originalGoalKickDownAdjust
	if state.VelocityY < 0 {
		table = originalGoalKickUpAdjust
	}
	state.VelocityX += int32(table[choice][0]) << 16
	state.VelocityY += int32(table[choice][1]) << 16
	m.Ball.applyOriginalState(state)
}

// Rule5 at1285E waits for the actual last-touch pointer established by the
// keeper's clearance, then releases all active actors to ordinary play.
func (m *Match) updateOriginalGoalKickRule() {
	if m.originalRuleState != 5 || m.Ball.LastTeam < 0 {
		return
	}
	for side := range m.Teams {
		for index := range m.Teams[side].Players {
			p := &m.Teams[side].Players[index]
			if p.originalState != 0 {
				p.originalState, p.originalDirectionHistory = 1, -1
			}
		}
	}
	m.originalRuleState = 0
}
