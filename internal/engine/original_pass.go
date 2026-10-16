package engine

import "math"

// OriginalPassState translates 0xbd32/0xbd9e for nine supplied random words.
// ReceiverDistance is the receiver's cached integer distance to the ball.
func OriginalPassState(passer, target OriginalBallState, receiverDistance, skill int, words [9]uint16) OriginalBallState {
	distance := originalSquareRoot(receiverDistance)
	if distance < 1 {
		distance = 1
	}
	denominator := ((100 - clampInt(skill, 0, 100)) << 7) + 1
	errorComponent := func(start int) int32 {
		sum := 0
		for _, word := range words[start : start+4] {
			sum += int(word) % denominator
		}
		sum -= 2 * (denominator - 1)
		return int32(sum*distance) << 2
	}
	dx, dy := target.X+errorComponent(0)-passer.X, target.Y+errorComponent(4)-passer.Y
	velocity := func(delta int32) int32 {
		scaled := delta >> 4
		quotient := scaled / int32(int16(distance))
		// DIVS.W leaves its destination unchanged on quotient overflow.
		// EXT.L then consumes that preserved low word, not a wrapped quotient.
		normalized := int16(scaled)
		if quotient >= -32768 && quotient <= 32767 {
			normalized = int16(quotient)
		}
		return int32(normalized)<<6 + int32(int16(delta>>16))*300
	}
	return OriginalBallState{VelocityX: velocity(dx), VelocityY: velocity(dy), Lift: int32(200+int(words[8]%100)) * 200}
}

func (m *Match) originalPass(team, index, target int) {
	p, q := &m.Teams[team].Players[index], &m.Teams[team].Players[target]
	var words [9]uint16
	for i := range words {
		words[i] = OriginalRandomWord(&m.originalRandomState)
	}
	position := func(value Vec2) OriginalBallState {
		return OriginalBallState{X: int32(math.Round(value.X * originalFixedScale)), Y: int32(math.Round(value.Y * originalFixedScale))}
	}
	launch := OriginalPassState(position(p.Pos), position(q.Pos), int(q.originalBallDistance), int(math.Round(p.Attributes.Passing*100)), words)
	state := originalBallState(m.Ball, 0)
	state.VelocityX, state.VelocityY, state.Lift = launch.VelocityX, launch.VelocityY, launch.Lift
	m.Ball.applyOriginalState(state)
	m.Ball.TrappedBy = -1
	m.originalBallOwner = team*11 + target
	m.Ball.LastTeam, m.Ball.LastPlayer = team, index
	m.originalLastShooter = team*11 + index
	p.lastTouch, p.touchCooldown = m.Tick, 15
	// 0xbe0a/0xbe14 stops the passer and enters the brief state-30
	// recovery. Kickoff explicitly replaces this with its state-32 wait.
	p.originalStep, p.originalState = 0, 30
	q.touchCooldown, q.originalOwnerTicks = 0, 100
	m.emit("pass", team, index, m.Ball.Pos)
}

// originalTrapPassTarget translates0xbb6c/0x10380. A trapped-ball release
// first uses the held facing, then either adjacent lane; the new joystick
// direction is used only when no teammate is available and the player shoots.
func (m *Match) originalTrapPassTarget(team, index int) int {
	p := &m.Teams[team].Players[index]
	var choices [8]int
	var distances [8]int32
	for direction := range choices {
		choices[direction], distances[direction] = -1, math.MaxInt32
	}
	for candidate := range m.Teams[team].Players {
		q := &m.Teams[team].Players[candidate]
		if candidate == index || q.originalState == 0 || q.originalBallDistance < 2500 || q.originalBallDistance > 810000 {
			continue
		}
		direction := OriginalDirectionIndex(Vec2{math.Floor(q.Pos.X) - math.Floor(p.Pos.X), math.Floor(q.Pos.Y) - math.Floor(p.Pos.Y)})
		if q.originalBallDistance <= distances[direction] {
			choices[direction], distances[direction] = candidate, q.originalBallDistance
		}
	}
	direction := OriginalDirectionIndex(p.Facing)
	if choices[direction] >= 0 {
		return choices[direction]
	}
	clockwise, anticlockwise := (direction+1)&7, (direction+7)&7
	if choices[clockwise] >= 0 {
		if choices[anticlockwise] < 0 || distances[clockwise] < distances[anticlockwise] {
			return choices[clockwise]
		}
	}
	return choices[anticlockwise]
}

func originalSector(position, target Vec2) int {
	delta := Vec2{math.Floor(target.X) - math.Floor(position.X), math.Floor(target.Y) - math.Floor(position.Y)}
	direction := OriginalDirectionIndex(delta)
	base := 0
	if math.Abs(delta.X) > math.Abs(delta.Y) {
		base = 1
	}
	if delta.X < 0 {
		base ^= 7
	}
	if delta.Y > 0 {
		base ^= 3
	}
	return 2*base + ((direction - base) & 7)
}

// originalPassingTarget reproduces the directional team/opponent groups and
// the goal-relative preference table used by the kickoff branch at0xe63c.
func (m *Match) originalPassingTarget(team, index int) int {
	p := &m.Teams[team].Players[index]
	goal := Vec2{456, 0}
	if m.AttackDirection(team) > 0 {
		goal.Y = 1392
	}
	sector := originalSector(p.Pos, goal)
	preferences := [16][8]int{
		{0, 1, 7, 2, 6, 3, 5, 4}, {1, 0, 2, 7, 3, 6, 4, 5}, {1, 2, 0, 3, 7, 4, 6, 5}, {2, 1, 3, 0, 4, 7, 5, 6},
		{2, 3, 1, 4, 0, 5, 7, 6}, {3, 2, 4, 1, 5, 0, 6, 7}, {3, 4, 2, 5, 1, 6, 0, 7}, {4, 3, 5, 2, 6, 1, 7, 0},
		{4, 5, 3, 6, 2, 7, 1, 0}, {5, 4, 6, 3, 7, 2, 0, 1}, {5, 6, 4, 7, 3, 0, 2, 1}, {6, 5, 7, 4, 0, 3, 1, 2},
		{6, 7, 5, 0, 4, 1, 3, 2}, {7, 6, 0, 5, 1, 4, 2, 3}, {7, 0, 6, 1, 5, 2, 4, 3}, {0, 7, 1, 6, 2, 5, 3, 4},
	}
	var choices [2][8]int
	var distances [2][8]int32
	for side := range choices {
		for d := range choices[side] {
			choices[side][d], distances[side][d] = -1, math.MaxInt32
		}
	}
	for side := 0; side < 2; side++ {
		actual := team
		if side == 1 {
			actual = 1 - team
		}
		for i, q := range m.Teams[actual].Players {
			if q.SentOff || actual == team && i == index || m.Config.Practice && actual == 1 && i != 0 || q.originalBallDistance > 160000 {
				continue
			}
			direction := OriginalDirectionIndex(q.Pos.Sub(p.Pos))
			if q.originalBallDistance < distances[side][direction] {
				choices[side][direction], distances[side][direction] = i, q.originalBallDistance
			}
		}
	}
	for _, direction := range preferences[sector] {
		candidate := choices[0][direction]
		// BCC0 rejects receivers below20 or above200 source units, unlike
		// the wider grouping radius used by BB04 before this final choice.
		if candidate < 0 || distances[0][direction] < 400 || distances[0][direction] > 40000 || choices[1][direction] >= 0 && distances[0][direction] >= distances[1][direction] {
			continue
		}
		p.Facing = OriginalDirection(direction)
		return candidate
	}
	return -1
}

func (m *Match) updateOriginalKickoffPlayer(team, index int) (Vec2, int) {
	p := &m.Teams[team].Players[index]
	if p.originalState == 32 {
		if m.Ball.LastTeam != team || m.Ball.LastPlayer != index {
			p.originalState, p.originalDirectionHistory = 1, -1
			p.AnimationPhase = 0
		}
		return Vec2{}, 0
	}
	if originalDistanceSquared(p.Pos, m.Ball.Pos) > 16 {
		p.Facing = originalTargetHeading(p, m.Ball.Pos)
		return p.Facing, originalPace(p)
	}
	m.originalBallOwner = team*11 + index
	m.Ball.LastTeam, m.Ball.LastPlayer = team, index
	// E630 records the kickoff actor even when its pass scan finds no receiver.
	m.originalLastShooter = team*11 + index
	if target := m.originalPassingTarget(team, index); target >= 0 {
		m.originalPass(team, index, target)
		p.originalState, p.originalDirectionHistory = 32, -1
		m.originalKickoffReset = true
	}
	return Vec2{}, 0
}

func (m *Match) updateOriginalAIOwner(team, index int) (Vec2, int) {
	p := &m.Teams[team].Players[index]
	pace := originalPace(p)
	if p.touchCooldown <= 0 && p.originalBallDistance <= 64 && int(m.Ball.Height) <= 12 {
		m.Ball.Vel, m.Ball.VerticalSpeed = Vec2{}, 0
		m.Ball.LastTeam, m.Ball.LastPlayer = team, index
		p.originalOwnerTicks, p.lastTouch = 300, m.Tick
		if m.Config.Human[team] && m.boundPrimaryPlayer(team) < 0 {
			m.Teams[team].selectOwnPlayer(index)
			return p.Facing, pace
		}
		goal := Vec2{456, -24}
		if m.AttackDirection(team) > 0 {
			goal.Y = 1416
		}
		distance := originalDistanceSquared(p.Pos, goal)
		far := distance > 208000
		if distance > 72900 && !far {
			// 0xdcd8 considers a longer shot on thirty-one remainders.
			far = OriginalRandomWord(&m.originalRandomState)%100 > 30
		}
		shoot := distance < 14400
		if !far && !shoot {
			shoot = int(OriginalRandomWord(&m.originalRandomState)%360)+60 < int(math.Round(p.Attributes.Shooting*100))
		}
		if shoot {
			m.originalAIAimedShot(team, index, goal)
			return p.Facing, pace
		}
		if !far {
			guide := m.originalBallGoalGuide
			if !m.originalBallDerivedReady {
				guide = originalGoalGuide(originalBallState(m.Ball, m.ballCurve))
			}
			// DD10..DD3E tests the integrated ball's three goal-mouth
			// guide words before falling back to possession lanes.
			if originalGoalGuideInMouth(guide) {
				m.originalAIGuidedShot(team, index, goal)
				return p.Facing, pace
			}
		}
		y := math.Floor(p.Pos.Y)
		if far && y >= 150 && y < 1242 || !far && y >= 50 && y < 1342 {
			return m.originalPossessionLane(team, index, goal)
		}
		// The wide/end-line branch at 0xdd7e chips toward the goal mouth.
		chipGoal := Vec2{456, 40}
		if m.AttackDirection(team) > 0 {
			chipGoal.Y = 1352
		}
		direction := OriginalDirectionIndex(chipGoal.Sub(p.Pos))
		p.Facing = OriginalDirection(direction)
		m.originalChip(team, index, direction)
		return p.Facing, pace
	}
	if p.originalOwnerTicks < 0 {
		m.originalBallOwner = -1
	}
	p.originalOwnerTicks--
	p.Facing = m.originalChaseHeading(p)
	return p.Facing, pace
}
