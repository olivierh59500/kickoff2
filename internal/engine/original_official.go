package engine

import "math"

// originalSetOfficialAction translates EBE8. Practice and disabled officials
// immediately satisfy the readiness gate; the normal referee must run its
// actual action before setting ready, rather than waiting an invented timer.
func (m *Match) originalSetOfficialAction(action int) {
	if m.Config.Practice || !m.originalOfficialsEnabled {
		m.originalOfficialReady = true
		return
	}
	p := &m.originalOfficial
	p.originalState, p.originalDirectionHistory, p.AnimationPhase = action, -1, 0
	m.originalOfficialGestureStarted = false
	m.originalOfficialReady = false
}

// updateOriginalOfficial translates ECFE/ED1E/ED30 and the B5E4 running
// accumulator. FCDE executes this actor after the ball and before footballers.
// Card/foul-specific referee actions have separate source handlers.
func (m *Match) updateOriginalOfficial() {
	if m.Config.Practice || !m.originalOfficialsEnabled || !m.originalHasOfficial {
		return
	}
	p := &m.originalOfficial
	step := 0
	switch p.originalState {
	case 0:
		_, next, arrived := m.originalRunToTarget(p, Vec2{384, 696})
		step = next
		if arrived {
			p.Facing = originalTargetHeading(p, m.Ball.Pos)
			m.originalOfficialReady = true
		}
	case 1:
		p.originalDirectionHistory = -1
		p.Facing = originalTargetHeading(p, m.Ball.Pos)
	case 2:
		step = 140
		p.Facing = originalTargetHeading(p, m.Ball.Pos)
		if originalDistanceSquared(p.Pos, m.Ball.Pos) <= 10000 {
			step = 0
			m.originalOfficialReady = true
		}
	case 3, 4, 5, 6:
		offender := m.originalFoul.Offender
		q := &m.Teams[offender/11].Players[offender%11]
		if !m.originalOfficialGestureStarted || p.originalState == 3 {
			_, next, arrived := m.originalRunToTarget(p, Vec2{math.Floor(q.Pos.X), math.Floor(q.Pos.Y) - 24})
			step = next
			if arrived {
				p.Facing = OriginalDirection(4)
				if p.originalState == 3 {
					m.originalOfficialReady = true
				} else {
					m.originalOfficialGestureStarted = true
					p.AnimationPhase = 0
				}
			}
		}
	default:
		return
	}
	p.originalStep = step
	if m.originalOfficialGestureStarted && p.originalState >= 4 && p.originalState <= 6 {
		if int16(p.AnimationPhase) < 0 {
			m.originalOfficialReady = true
		} else {
			old := p.AnimationPhase
			p.AnimationPhase += 128
			if int(old>>8) >= 36+4*(p.originalState-4) {
				p.AnimationPhase = 65535
			}
		}
		p.Vel = Vec2{}
		return
	}
	phase := p.AnimationPhase + uint16(step*2)
	if phase>>8 >= 38 {
		phase -= 0x2600
	}
	p.AnimationPhase = phase
	p.Vel = p.Facing.Mul(float64(step) * 1000 * TPS / originalFixedScale)
	p.Pos = p.Pos.Add(p.Vel.Mul(1.0 / TPS))
}
