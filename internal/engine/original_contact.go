package engine

import "math"

// originalPlayerContact translates the six direction-table reactions at0x11a90.
// Source reactions change heading, possession, timers or action; none separate
// overlapping player coordinates geometrically.
func (m *Match) originalPlayerContact(team, index, owner int) {
	// 11A90 returns before every collision reaction while a foul is active.
	if m.originalFoul.Active {
		return
	}
	p, q := &m.Teams[team].Players[index], &m.Teams[owner/11].Players[owner%11]
	if q.Keeper {
		return
	}
	if q.originalStep == 0 {
		m.originalBallOwner = team*11 + index
		p.originalOwnerTicks = 100
		return
	}
	if p.originalStep == 0 {
		return
	}
	bearing := OriginalDirectionIndex(Vec2{math.Floor(q.Pos.X) - math.Floor(p.Pos.X), math.Floor(q.Pos.Y) - math.Floor(p.Pos.Y)})
	enemyDirection := OriginalDirectionIndex(q.Facing)
	relativeBearing := (bearing - enemyDirection) & 7
	relativeFacing := (OriginalDirectionIndex(p.Facing) - enemyDirection) & 7
	table := [64]int{3, 3, 0, 0, 0, 0, 0, 3, 3, 3, 3, 0, 0, 0, 0, 0, 0, 4, 3, 5, 0, 0, 0, 0, 1, 1, 2, 2, 2, 1, 1, 1, 1, 1, 1, 2, 2, 2, 1, 1, 1, 1, 1, 1, 2, 2, 2, 1, 0, 0, 0, 0, 0, 4, 3, 5, 3, 0, 0, 0, 0, 0, 3, 3}
	switch table[relativeBearing*8+relativeFacing] {
	case 1:
		m.originalBallOwner = -1
		q.touchCooldown, q.originalState = 10, 30
	case 2:
		selfRank := int(math.Round(p.Attributes.Tackling*100))<<16 | int(math.Round(p.Attributes.Resilience*100))
		enemyRank := int(math.Round(q.Attributes.Tackling*100))<<16 | int(math.Round(q.Attributes.Resilience*100))
		if enemyRank < selfRank {
			m.originalBallOwner = team*11 + index
			p.originalOwnerTicks, p.originalStep = 100, 0
			q.touchCooldown, q.originalState = 10, 14
			q.originalRecoveryLimit = uint16(20+OriginalRandomWord(&m.originalRandomState)%20) << 8
			q.AnimationPhase = 0
			// 11B86 invokes the same impact sequence as a successful tackle.
			m.emit("player_contact", team, index, q.Pos)
		} else {
			m.originalBallOwner = owner
			q.originalOwnerTicks = 100
			p.touchCooldown, p.originalState, p.originalStep = 10, 30, 0
		}
	case 3:
		p.touchCooldown, p.originalState = 10, 30
	case 4:
		p.Facing = OriginalDirection(OriginalDirectionIndex(p.Facing) - 1)
	case 5:
		p.Facing = OriginalDirection(OriginalDirectionIndex(p.Facing) + 1)
	}
}

// originalAIBallDecision follows DB5E..DBAA before possession/contact handling.
// Nearby actors and the nearest actor proceed directly. A distant second actor
// waits within thirty units of the nearest; other distant actors keep formation.
func (m *Match) originalAIBallDecision(team, index, closest, second int) (approach, wait bool) {
	p := &m.Teams[team].Players[index]
	zone := clampInt(int(m.Ball.Pos.X/304), 0, 2)*4 + clampInt(int(m.Ball.Pos.Y/348), 0, 3)
	nearby := p.originalBallDistance < 2500 || (zone == 4 || zone == 7) && p.originalBallDistance < 10000
	if nearby || index == closest {
		return true, false
	}
	if index != second {
		return false, false
	}
	return true, closest >= 0 && originalDistanceSquared(p.Pos, m.Teams[team].Players[closest].Pos) <= 900
}

func (m *Match) originalAIContact(team, index, closest, second int) (Vec2, int, bool) {
	p := &m.Teams[team].Players[index]
	owner := m.originalBallOwner
	approach, wait := m.originalAIBallDecision(team, index, closest, second)
	if owner < 0 || owner/11 == team || p.originalState != 1 || !approach || wait {
		return Vec2{}, 0, false
	}
	if p.originalBallDistance <= 1600 {
		word := OriginalRandomWord(&m.originalRandomState) % 1500
		enter := int(word) < p.originalAggressionThreshold
		if enter && int(word) >= p.originalAggressionThreshold>>1 {
			// DBDE..DC06 suppresses the upper half of the tackle interval
			// when the owner is straight ahead and closer than the cached ball.
			q := &m.Teams[owner/11].Players[owner%11]
			bearing := OriginalDirectionIndex(Vec2{math.Floor(q.Pos.X) - math.Floor(p.Pos.X), math.Floor(q.Pos.Y) - math.Floor(p.Pos.Y)})
			if bearing == OriginalDirectionIndex(p.Facing) && originalDistanceSquared(p.Pos, q.Pos) <= p.originalBallDistance {
				enter = false
			}
		}
		if enter {
			p.originalState, p.originalDirectionHistory = 12, -1
			p.tackleTicks, p.ActionTicks = 0, 0
			p.originalTackleHit = false
			p.originalStep = originalPace(p) * 2
			m.emit("tackle", team, index, p.Pos)
			return p.Facing, p.originalStep, true
		}
	}
	q := &m.Teams[owner/11].Players[owner%11]
	if originalDistanceSquared(p.Pos, q.Pos) > 576 {
		return Vec2{}, 0, false
	}
	p.originalStep = originalPace(p)
	m.originalPlayerContact(team, index, owner)
	return p.Facing, p.originalStep, true
}

// updateOriginalTackle follows the state-12 decay and low-ball strike at
// E1AC..E230 and the distinct human1040C/10466 gates. Its sprite is static;
// motion uses the decaying source word. Referee adjudication is separate.
func (m *Match) updateOriginalTackle(team, index int) (Vec2, int) {
	p := &m.Teams[team].Players[index]
	in, _, controlled := m.playerInput(team, index)
	human := controlled && m.Config.Human[team]
	if human {
		p.originalDirectionHistory = -1
	}
	step := p.originalStep - (p.originalStep >> 5)
	if step <= 32 {
		// Human10428 retains action12 until fire is released; CPU E1BE
		// returns immediately to its ordinary action at the same threshold.
		if human && in.Fire {
			return Vec2{}, 0
		}
		p.originalState, p.originalDirectionHistory = 1, -1
		p.AnimationPhase = 0
		return Vec2{}, 0
	}
	tackling := int(math.Round(p.Attributes.Tackling * 100))
	if owner := m.originalBallOwner; (human || step >= 120) && owner >= 0 && owner/11 != team {
		q := &m.Teams[owner/11].Players[owner%11]
		distance := 120 - tackling
		if human {
			distance = 100
		}
		if originalDistanceSquared(p.Pos, q.Pos) <= int32(distance) {
			m.originalTackleContact(team, index, owner, step)
		}
	}
	contactDistance := tackling * 2
	if human {
		// 10466 uses a fixed64 cached distance, unlike E202's skill*2.
		contactDistance = 64
	}
	if p.originalBallDistance <= int32(contactDistance) && int(m.Ball.Height) <= 12 && p.touchCooldown <= 0 {
		state := originalBallState(m.Ball, m.ballCurve)
		delta := p.Facing.Mul(float64(step) * 1000)
		state.VelocityX, state.VelocityY, state.Lift = int32(math.Round(delta.X)), int32(math.Round(delta.Y)), 80000
		m.Ball.applyOriginalState(state)
		m.originalBallOwner = -1
		m.Ball.LastTeam, m.Ball.LastPlayer = team, index
		m.originalLastShooter = team*11 + index
		p.touchCooldown, p.lastTouch, p.originalTackleHit = 15, m.Tick, true
		m.emit("tackle_touch", team, index, m.Ball.Pos)
	}
	return p.Facing, step
}
