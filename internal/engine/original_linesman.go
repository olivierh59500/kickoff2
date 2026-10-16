package engine

import "math"

// updateOriginalLinesman translates EC40..ECC8. The assistant tracks the ball
// on the camera's nearer touchline and holds the source flag after a throw.
func (m *Match) updateOriginalLinesman() {
	if !m.originalHasLinesman || m.Config.Practice || !m.originalOfficialsEnabled {
		return
	}
	p := &m.originalLinesman
	move, step, arrived := Vec2{}, 0, true
	if m.originalGoalFrame.OutKind != 1 {
		m.originalLinesmanFlag = -1
		x := -32.0
		if m.originalCameraX>>16 >= 456 {
			x = 948
		}
		p.Pos.X = originalCoordinateWord(p.Pos.X, int(x))
		move, step, arrived = m.originalRunToTarget(p, Vec2{math.Floor(p.Pos.X), clamp(math.Floor(m.Ball.Pos.Y), 24, 1368)})
		if arrived {
			p.Facing = OriginalDirection(0)
			if x >= 456 {
				p.Facing = OriginalDirection(3)
			}
		}
	} else if m.originalLinesmanFlag < 0 {
		move, step, arrived = m.originalRunToTarget(p, Vec2{math.Floor(p.Pos.X), float64(m.originalGoalFrame.OutY)})
		if arrived {
			direction := 1
			if p.Pos.X >= 456 {
				direction = 4
			}
			// ECB2 uses the receiving team's boundary pointer, initialized
			// by the out detector before the throw manager runs.
			if m.Ball.LastTeam >= 0 && m.AttackDirection(1-m.Ball.LastTeam) < 0 {
				direction++
			}
			p.Facing = OriginalDirection(direction)
			m.originalLinesmanFlag = direction
		}
	}
	p.originalStep = step
	if !arrived {
		p.AnimationPhase += uint16(step * 2)
		if p.AnimationPhase>>8 >= 38 {
			p.AnimationPhase -= 0x2600
		}
		p.Pos = p.Pos.Add(move.Mul(float64(step) * 1000 / 65536))
	}
	p.Vel = move.Mul(float64(step) * 1000 * TPS / 65536)
	code := 807 + OriginalDirectionIndex(p.Facing)
	if !arrived {
		code = 784 + OriginalDirectionIndex(p.Facing)*2 + OriginalRunningFrame(p.AnimationPhase, false)
	}
	m.originalVideoSprite(p.Pos, code, 0)
}
