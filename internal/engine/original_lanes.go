package engine

import "math"

var originalGoalPreferences = [16][8]int{
	{0, 1, 7, 2, 6, 3, 5, 4}, {1, 0, 2, 7, 3, 6, 4, 5}, {1, 2, 0, 3, 7, 4, 6, 5}, {2, 1, 3, 0, 4, 7, 5, 6},
	{2, 3, 1, 4, 0, 5, 7, 6}, {3, 2, 4, 1, 5, 0, 6, 7}, {3, 4, 2, 5, 1, 6, 0, 7}, {4, 3, 5, 2, 6, 1, 7, 0},
	{4, 5, 3, 6, 2, 7, 1, 0}, {5, 4, 6, 3, 7, 2, 0, 1}, {5, 6, 4, 7, 3, 0, 2, 1}, {6, 5, 7, 4, 0, 3, 1, 2},
	{6, 7, 5, 0, 4, 1, 3, 2}, {7, 6, 0, 5, 1, 4, 2, 3}, {7, 0, 6, 1, 5, 2, 4, 3}, {0, 7, 1, 6, 2, 5, 3, 4},
}

// originalPossessionLane translates 0xdd8e..0xde3a: compare the best pass
// with the first unoccupied direction, using the original sector priority.
func (m *Match) originalPossessionLane(team, index int, goal Vec2) (Vec2, int) {
	p := &m.Teams[team].Players[index]
	pace := originalPace(p)
	p.Facing = originalTargetHeading(p, goal)
	preferences := originalGoalPreferences[originalSector(p.Pos, goal)]
	var choices [2][8]int
	var distances [2][8]int32
	for side := range choices {
		for direction := range choices[side] {
			choices[side][direction], distances[side][direction] = -1, math.MaxInt32
		}
	}
	// 0xbb1c groups all active actors by their eight-direction bearing.
	// It compares their cached distance to the ball, not actor-to-actor distance.
	for side := range choices {
		actualTeam := team
		if side == 1 {
			actualTeam = 1 - team
		}
		for candidate := range m.Teams[actualTeam].Players {
			q := &m.Teams[actualTeam].Players[candidate]
			if q.originalState == 0 || q.SentOff || actualTeam == team && candidate == index || q.originalBallDistance > 160000 {
				continue
			}
			direction := OriginalDirectionIndex(Vec2{math.Floor(q.Pos.X) - math.Floor(p.Pos.X), math.Floor(q.Pos.Y) - math.Floor(p.Pos.Y)})
			if q.originalBallDistance <= distances[side][direction] {
				choices[side][direction], distances[side][direction] = candidate, q.originalBallDistance
			}
		}
	}
	passPriority, passDirection, receiver := 8, 0, -1
	for priority, direction := range preferences {
		if choices[0][direction] < 0 || distances[0][direction] < 4900 || choices[1][direction] >= 0 && distances[0][direction] >= distances[1][direction] {
			continue
		}
		passPriority, passDirection, receiver = priority, direction, choices[0][direction]
		break
	}
	if passPriority < 3 {
		p.Facing = OriginalDirection(passDirection)
		m.originalPass(team, index, receiver)
		return Vec2{}, 0
	}
	// 0xbace excludes dribbling directions leading out of the pitch.
	var boundary uint8
	x, y := math.Floor(p.Pos.X), math.Floor(p.Pos.Y)
	if x <= 100 {
		boundary |= 0xe0
	} else if x >= 812 {
		boundary |= 0x07
	}
	if y <= 90 {
		boundary |= 0x83
	} else if y >= 1302 {
		boundary |= 0x38
	}
	dribblePriority := 8
	for priority, direction := range preferences {
		if boundary&(1<<direction) == 0 && choices[1][direction] < 0 {
			dribblePriority = priority
			p.Facing = OriginalDirection(direction)
			break
		}
	}
	if dribblePriority < 3 || dribblePriority < 5 && passPriority >= 5 {
		// 0xde10 reduces a dribbler's step near its own goal.
		if m.AttackDirection(team) < 0 && y <= 216 || m.AttackDirection(team) > 0 && y >= 1176 {
			pace -= 40
		}
		state := originalBallState(m.Ball, m.ballCurve)
		launch := OriginalDribbleState(OriginalDirectionIndex(p.Facing), pace)
		state.VelocityX, state.VelocityY, state.Lift = launch.VelocityX, launch.VelocityY, launch.Lift
		m.Ball.applyOriginalState(state)
		m.originalLastShooter = team*11 + index
		p.touchCooldown = 15
		m.emit("touch", team, index, m.Ball.Pos)
		return p.Facing, pace
	}
	if passPriority < 7 && receiver >= 0 {
		p.Facing = OriginalDirection(passDirection)
		m.originalPass(team, index, receiver)
		return Vec2{}, 0
	}
	// 0xddec delegates a completely blocked lane to the clearance routine.
	m.originalKeeperKick(team, p)
	return p.Facing, pace
}
