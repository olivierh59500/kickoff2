package engine

import "math"

// OriginalAimedShotState translates C2F0..C376. The CPU aims continuously at
// the side opposite the visible keeper, then consumes C266's six lift draws.
// TargetFraction preserves the high word of incoming D1 through MOVE.W/SWAP.
func OriginalAimedShotState(ball OriginalBallState, keeperX, skill, attack int, targetFraction uint16, words [6]uint16) OriginalBallState {
	x, y := 456, 0
	if keeperX >= 408 && keeperX <= 504 {
		x = 480
		if keeperX >= 456 {
			x = 432
		}
	}
	if attack > 0 {
		y = 1392
	}
	dx, dy := (int32(x<<16)|int32(targetFraction))-ball.X, int32(y<<16)-ball.Y
	sx, sy := int32(int16(dx>>12)), int32(int16(dy>>12))
	root := originalSquareRoot(int(uint32(sx*sx + sy*sy)))
	if root == 0 {
		// An exact zero vector would enter the original DIVS-zero exception;
		// ordinary owner dispatch cannot award a shot at that scored point.
		return ball
	}
	normalize := func(delta int32) int32 {
		quotient := delta / int32(int16(root))
		word := int16(delta)
		if quotient >= -32768 && quotient <= 32767 {
			word = int16(quotient)
		}
		return int32(word) << 7
	}
	ball.VelocityX, ball.VelocityY = normalize(dx), normalize(dy)
	lift := int16(106)
	for index, word := range words {
		error := int16(int(word) % (101 - clampInt(skill, 0, 100)))
		if index&1 == 0 {
			lift += error
		} else {
			lift -= error
		}
	}
	ball.Lift = int32(lift) * 1000
	return ball
}

func (m *Match) originalAIAimedShot(team, index int, goal Vec2) {
	p := &m.Teams[team].Players[index]
	var words [6]uint16
	for at := range words {
		words[at] = OriginalRandomWord(&m.originalRandomState)
	}
	// BC32 leaves the squared integer Y difference in D1. The intervening
	// random routine preserves it; C2F0 retains that register's high word.
	dy := int32(int16(int(goal.Y) - int(math.Floor(p.Pos.Y))))
	fraction := uint16(uint32(dy*dy) >> 16)
	state := OriginalAimedShotState(originalBallState(m.Ball, m.ballCurve), int(math.Floor(m.Teams[1-team].Players[0].Pos.X)), int(math.Round(p.Attributes.Shooting*100)), m.AttackDirection(team), fraction, words)
	m.Ball.applyOriginalState(state)
	m.Ball.TrappedBy, m.originalBallOwner = -1, -1
	m.Ball.LastTeam, m.Ball.LastPlayer = team, index
	m.originalLastShooter = team*11 + index
	p.originalState, p.touchCooldown, p.lastTouch = 30, 15, m.Tick
	m.emit("shot", team, index, m.Ball.Pos)
}

// originalGoalGuide is the three-word geometric guide at A8EA..A910. These
// are integrated ball coordinates, retained before any actor changes velocity.
func originalGoalGuide(ball OriginalBallState) [3]int16 {
	x, y := int16(ball.X>>16), int16(ball.Y>>16)
	if y >= 696 {
		y = 1392 - y
	}
	return [3]int16{x + y, x, x - y}
}

// Preserve DD10..DD3E's ordered signed-word comparisons, including the
// different inclusive limits used for the three guide words.
func originalGoalGuideInMouth(guide [3]int16) bool {
	if guide[2] > 504 {
		return false
	}
	if guide[2] > 408 {
		return true
	}
	if guide[0] < 408 {
		return false
	}
	if guide[0] < 504 {
		return true
	}
	return guide[1] >= 408 && guide[1] <= 504
}

// originalAIGuidedShot follows DD52->C46C->C474. C46C establishes the
// goal-sector heading, then the ordinary ten-draw launch enters action30.
// Its CPU caller preserves the current step and never starts human aftertouch.
func (m *Match) originalAIGuidedShot(team, index int, goal Vec2) {
	p := &m.Teams[team].Players[index]
	p.Facing = originalTargetHeading(p, goal)
	direction := originalGoalPreferences[originalSector(p.Pos, goal)][0]
	launch := m.originalShot(OriginalDirection(direction), p.Attributes.Shooting)
	state := originalBallState(m.Ball, m.ballCurve)
	state.VelocityX, state.VelocityY, state.Lift = launch.VelocityX, launch.VelocityY, launch.Lift
	m.Ball.applyOriginalState(state)
	m.Ball.TrappedBy, m.originalBallOwner = -1, -1
	m.Ball.LastTeam, m.Ball.LastPlayer = team, index
	m.originalLastShooter = team*11 + index
	p.originalState, p.touchCooldown, p.lastTouch = 30, 15, m.Tick
	m.emit("shot", team, index, m.Ball.Pos)
}
