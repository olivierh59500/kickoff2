package engine

import "math"

// OriginalFoulRestartState retains13102's taker/camera pair and the human
// free-kick/penalty words. The match manager continues to use source rule IDs.
type OriginalFoulRestartState struct {
	Taker                           int
	CameraX, CameraY                int32
	ControllerLocked                bool
	Started, Height, Offset         int16
	Lateral, Vertical               int16
	Input, Heading                  int16
	PenaltyHeld                     uint16
	PenaltyMarker                   bool
	PenaltyX, PenaltyY, PenaltyStep int
	PenaltyDrawX, PenaltyDrawY      int
}

// originalFoulPenalty follows1308A's asymmetric area endpoints. In particular,
// upperY216 is outside the area while lowerY1176 is inside it.
func (m *Match) originalFoulPenalty() bool {
	x, y := int(m.originalFoul.Pos.X), int(m.originalFoul.Pos.Y)
	if y >= 216 && y < 1176 || x < 192 || x > 720 {
		return false
	}
	if m.AttackDirection(m.originalFoul.Team) > 0 {
		return y > 696
	}
	return y <= 696
}

func (m *Match) originalFoulClose() bool {
	goal := Vec2{456, 0}
	if m.AttackDirection(m.originalFoul.Team) > 0 {
		goal.Y = 1392
	}
	return originalDistanceSquared(m.originalFoul.Pos, goal) <= 176400
}

// originalDispatchFoulRestart is129EA, also reached after injury replacement
// at12A66. The penalty manager stages the spot on the following update.
func (m *Match) originalDispatchFoulRestart() {
	if m.originalFoulPenalty() {
		m.originalRuleState = 33
	} else if m.originalFoulClose() {
		m.originalStageCloseFreeKick()
	} else {
		p := &m.Teams[m.originalFoul.Victim/11].Players[m.originalFoul.Victim%11]
		p.originalState, p.originalDirectionHistory = 28, -1
		m.originalRuleState = 8
	}
}

func (m *Match) originalAssignActiveState(state int) {
	for team := range m.Teams {
		for index := range m.Teams[team].Players {
			p := &m.Teams[team].Players[index]
			if p.originalState != 0 {
				p.originalState, p.originalDirectionHistory = state, -1
			}
		}
	}
}

func (m *Match) originalPlaceRestartActor(team, index int, target Vec2) {
	p := &m.Teams[team].Players[index]
	p.originalState, p.originalDirectionHistory, p.originalRestartTarget = 55, -1, target
	// 13150/131B8 explicitly reactivate retained controller bindings, even
	// after removal. Keep the booking record while permitting state 55 to run.
	p.SentOff = false
}

// originalStageCloseFreeKick follows13102..13300, including the source's
// backwards scan from actor9 and five wall offsets(-40,-24,-8,8,24).
func (m *Match) originalStageCloseFreeKick() {
	foul, restart := &m.originalFoul, &m.originalFoulRestart
	team := foul.Team
	m.originalAssignActiveState(60)
	taker := foul.Victim % 11
	if fixed := m.boundPrimaryPlayer(team); fixed >= 0 {
		taker = fixed
	}
	behind := 1.0
	if m.AttackDirection(team) > 0 {
		behind = -1
	}
	m.originalPlaceRestartActor(team, taker, foul.Pos.Add(Vec2{Y: behind * 60}))
	restart.Taker = team*11 + taker
	backwards := func() int {
		for index := 9; index >= 0; index-- {
			if m.Teams[team].Players[index].originalState == 60 {
				return index
			}
		}
		return -1
	}
	first := m.boundPartnerPlayer(team)
	if first < 1 {
		first = backwards()
	}
	side := 36.0
	if foul.Pos.X > 456 {
		side = -side
	}
	if first >= 0 {
		m.originalPlaceRestartActor(team, first, foul.Pos.Add(Vec2{X: side, Y: behind * 24}))
	}
	if second := backwards(); second >= 0 {
		m.originalPlaceRestartActor(team, second, foul.Pos.Add(Vec2{X: -side, Y: behind * 24}))
	}
	goal := Vec2{456, 0}
	if foul.Pos.Y >= 696 {
		goal.Y = 1392
	}
	dx, dy := int(goal.X-foul.Pos.X), int(goal.Y-foul.Pos.Y)
	distance := originalSquareRoot(dx*dx + dy*dy)
	if distance != 0 {
		x, y := int(foul.Pos.X)+108*dx/distance, int(foul.Pos.Y)+108*dy/distance
		restart.CameraX = int32(x)<<16 | int32(uint16(restart.CameraX))
		restart.CameraY = int32(y)<<16 | int32(uint16(restart.CameraY))
		wall := 0
		for index := 1; index < 11 && wall < 5; index++ {
			if m.Teams[1-team].Players[index].originalState == 0 {
				continue
			}
			offset := -40 + wall*16
			m.originalPlaceRestartActor(1-team, index, Vec2{float64(x + dy*offset/distance), float64(y - dx*offset/distance)})
			wall++
		}
	}
	m.Ball.LastTeam, m.Ball.LastPlayer = -1, -1
	m.originalRuleState, m.originalRuleWait = 46, 100
	m.Restart = Restart{Kind: FreeKick, Team: team, Player: taker, Pos: foul.Pos}
	// 13102 preserves the current HUD text/timer and requests19B9E's sample12.
	m.emit("set_piece_ready", team, taker, foul.Pos)
}

// originalStageFoulPenalty translates12F08; only the coordinate high words
// change, retaining both observed fractions and the existing curve long.
func (m *Match) originalStageFoulPenalty() {
	p := &m.Teams[m.originalFoul.Victim/11].Players[m.originalFoul.Victim%11]
	state := p.originalState
	m.originalClockRunning = false
	m.originalAssignActiveState(43)
	p.originalState = state
	ball := originalBallState(m.Ball, m.ballCurve)
	y := 136
	if m.AttackDirection(m.originalFoul.Team) > 0 {
		y = 1256
	}
	ball.X = 456<<16 | int32(uint16(ball.X))
	ball.Y = int32(y)<<16 | int32(uint16(ball.Y))
	ball.Height, ball.VelocityX, ball.VelocityY, ball.Lift = 0, 0, 0, 0
	m.Ball.applyOriginalState(ball)
	m.originalGoalFrame.OutKind, m.originalBallDrawEnabled = 0, true
	m.originalRuleState = 31
	m.Restart = Restart{Kind: Penalty, Team: m.originalFoul.Team, Player: m.originalFoul.Victim % 11, Pos: m.Ball.Pos}
}

func (m *Match) originalPrepareFoulPenalty() {
	p := &m.Teams[m.originalFoul.Victim/11].Players[m.originalFoul.Victim%11]
	p.originalState, p.originalDirectionHistory = 41, -1
	m.originalRuleState, m.originalRuleWait = 32, 100
	m.originalCameraFixedX, m.originalCameraFixedY = 456<<16, 80<<16
	if m.Ball.Pos.Y >= 696 {
		m.originalCameraFixedY = 1312 << 16
	}
	m.originalCameraTarget = -2
	m.notice("PENALTY!", 5000)
	// 12ED0 skips1382E entirely during the final penalty contest.
	if !m.originalPeriod.PenaltyContest {
		m.emit("set_piece_ready", m.originalFoul.Team, m.originalFoul.Victim%11, m.Ball.Pos)
	}
}

func (m *Match) originalFinishFoulRestart(actor int) {
	p := &m.Teams[actor/11].Players[actor%11]
	state := p.originalState
	m.originalAssignActiveState(1)
	p.originalState = state
	m.originalFoul.Active, m.originalRuleState, m.originalCameraTarget = false, 0, -1
	m.originalFoul.LostTicks += m.originalClock - m.originalFoul.StoppageClock
	m.originalFoul.StoppageClock = 0
	m.originalFoulRestart.ControllerLocked = false
}

func (m *Match) updateOriginalFoulRestartRule() {
	switch m.originalRuleState {
	case 33:
		m.originalStageFoulPenalty()
	case 31:
		m.originalPrepareFoulPenalty()
	case 32:
		if m.originalActorsStopped() {
			m.originalRuleWait--
			if m.originalRuleWait < 0 {
				p := &m.Teams[m.originalFoul.Victim/11].Players[m.originalFoul.Victim%11]
				p.originalState, p.originalDirectionHistory = 42, -1
				m.originalFoul.Active, m.originalRuleState = false, 34
			}
		}
	case 34:
		ballY, cameraY := int(math.Floor(m.Ball.Pos.Y)), int(m.originalCameraFixedY>>16)
		if ballY >= 696 {
			ballY = 1392 - ballY
		}
		if cameraY >= 696 {
			cameraY = 1392 - cameraY
		}
		if ballY < cameraY {
			m.originalRuleState, m.originalClockRunning, m.originalCameraTarget = 0, true, -1
			m.originalFoul.LostTicks += m.originalClock - m.originalFoul.StoppageClock
			m.originalFoul.StoppageClock = 0
			if m.originalPeriod.PenaltyContest {
				m.originalRuleState, m.originalRuleWait = 35, 50
				m.originalFoul.LostTicks = 0
			}
		}
	case 46:
		if m.originalActorsStopped() {
			m.originalRuleWait--
			if m.originalRuleWait <= 0 {
				restart := &m.originalFoulRestart
				p := &m.Teams[restart.Taker/11].Players[restart.Taker%11]
				p.originalState, p.originalDirectionHistory = 56, -1
				if m.Config.Human[restart.Taker/11] && m.boundPrimaryPlayer(restart.Taker/11) < 0 {
					m.Teams[restart.Taker/11].selectOwnPlayer(restart.Taker % 11)
				}
				restart.ControllerLocked = true
				restart.CameraX = int32(int16(int16(restart.CameraX>>16)+int16(math.Floor(m.Ball.Pos.X)))>>1)<<16 | int32(uint16(restart.CameraX))
				restart.CameraY = int32(int16(int16(restart.CameraY>>16)+int16(math.Floor(m.Ball.Pos.Y)))>>1)<<16 | int32(uint16(restart.CameraY))
				m.originalRuleState, m.originalCameraTarget = 47, -4
			}
		}
	case 47:
		if m.Ball.LastTeam*11+m.Ball.LastPlayer == m.originalFoulRestart.Taker {
			m.originalFinishFoulRestart(m.originalFoulRestart.Taker)
		}
	}
}

// updateOriginalPenaltyParticipant follows E932's120/125 distance band and
// the asymmetric216/232 and1176/1160 area-edge comparisons.
func (m *Match) updateOriginalPenaltyParticipant(team, index int) (Vec2, int) {
	p := &m.Teams[team].Players[index]
	y, distance := math.Floor(p.Pos.Y), p.originalBallDistance
	move, stop := false, false
	direction := 4
	if m.Ball.Pos.Y > 696 {
		direction = 0
		move, stop = distance < 14400 || y > 1176, distance < 15625 || y > 1160
	} else {
		move, stop = distance < 14400 || y < 216, distance < 15625 || y < 232
	}
	if move {
		p.Facing, p.originalDirectionHistory = OriginalDirection(direction), -1
		return p.Facing, originalPace(p)
	}
	if stop {
		// E9DA calls B82A/B832, retaining its turning hysteresis even while
		// the penalty-area participant has a zero movement step.
		p.Facing = originalTargetHeading(p, m.Ball.Pos)
		return Vec2{}, 0
	}
	return m.originalTacticRun(team, index)
}

// OriginalFoulRestartPresentation exposes announcements and the stationary
// penalty arrow to the renderer/input observer, not hidden tactical decisions.
func (m *Match) OriginalFoulRestartPresentation() (RestartKind, int, bool) {
	if m.originalRuleState == 46 || m.originalRuleState == 47 {
		return FreeKick, m.originalFoul.Team, true
	}
	if m.originalRuleState >= 31 && m.originalRuleState <= 34 {
		return Penalty, m.originalFoul.Team, true
	}
	return KickOff, 0, false
}

// OriginalPenaltyMarkerVisual returns the arrow position actually drawn by
// E8B2 before advancing its oscillator, including the opposite-end sprite bank.
func (m *Match) OriginalPenaltyMarkerVisual() (Vec2, int, bool) {
	position, visible := Vec2{}, false
	if m.originalFoulRestart.PenaltyMarker {
		position = Vec2{float64(m.originalFoulRestart.PenaltyDrawX), float64(m.originalFoulRestart.PenaltyDrawY)}
		visible = true
	} else if (m.Phase == SetPiece || m.Phase == Shootout) && m.Restart.Kind == Penalty && m.Restart.PenaltyAimStep != 0 {
		position = Vec2{float64(m.Restart.PenaltyAimX), m.Restart.Pos.Y - 50}
		if m.Restart.Pos.Y > 696 {
			position.Y += 100
		}
		visible = true
	}
	index := 96
	if m.Ball.Pos.Y >= 696 {
		index += 100
	}
	return position, index, visible
}
