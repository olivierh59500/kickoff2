package engine

import "math"

func (m *Match) beginRestart(kind RestartKind, team int, pos Vec2, countdown int) {
	m.clearBallControl()
	m.Phase = SetPiece
	if kind == KickOff {
		m.Phase = Kickoff
	}
	m.Ball = Ball{Pos: pos, LastTeam: -1, LastPlayer: -1, TrappedBy: -1}
	m.Restart = Restart{Kind: kind, Team: team, Pos: pos, Countdown: countdown, Player: 10}
	for t := range m.Teams {
		m.Teams[t].Formation = m.Teams[t].queuedFormation
		for i := range m.Teams[t].Players {
			m.Teams[t].Players[i].Vel = Vec2{}
			if i > 0 && !(m.Config.Practice && t == 1) {
				if target, ok := m.restartTacticPosition(t, i, kind, t == team, pos, m.AttackDirection(t)); ok {
					m.Teams[t].Players[i].Pos = target
				}
			}
		}
	}
	if kind == GoalKick {
		m.Restart.Player = 0
	} else {
		m.Restart.Player = m.nearest(team, -1)
		if m.Restart.Player < 1 {
			m.Restart.Player = 1
		}
		if kind == Penalty {
			m.Restart.Player = m.penaltyTaker(team)
		}
	}
	p := &m.Teams[team].Players[m.Restart.Player]
	p.Facing = Vec2{0, float64(m.AttackDirection(team))}
	if kind == Corner || kind == ThrowIn {
		p.Facing = Vec2{PitchWidth / 2, PitchHeight / 2}.Sub(pos).Unit()
	}
	p.Pos = pos.Sub(p.Facing.Mul(17))
	if m.Config.Human[team] {
		m.Teams[team].selectOwnPlayer(m.Restart.Player)
	} else {
		m.Teams[team].selectOwnPlayer(-1)
	}
	if kind == Penalty {
		if m.Config.Human[team] {
			m.Restart.PenaltyAimX, m.Restart.PenaltyAimStep = 456, 3
		}
		m.arrangePenalty(team)
	} else if kind == FreeKick {
		goal := Vec2{PitchWidth / 2, 0}
		if m.AttackDirection(team) == 1 {
			goal.Y = PitchHeight
		}
		front := goal.Sub(pos).Unit()
		side := Vec2{-front.Y, front.X}
		for i := 1; i <= 3; i++ {
			q := &m.Teams[1-team].Players[i]
			if !q.SentOff {
				q.Pos = pos.Add(front.Mul(90)).Add(side.Mul(float64(i-2) * 21))
			}
		}
	}
	m.notice(kind.String(), TPS*3)
	m.emit("whistle", team, m.Restart.Player, pos)
}

func (m *Match) updateRestart(input [2]Input) {
	if m.Restart.Kind == Penalty {
		m.updateOriginalPenaltyRestart(input)
		return
	}
	r := &m.Restart
	r.Countdown--
	in := input[r.Team]
	p := &m.Teams[r.Team].Players[r.Player]
	if r.Kind == Corner && !r.Armed {
		if in.CornerChoice != nil {
			r.CornerChoice = clampInt(*in.CornerChoice, 0, 8)
		} else if in.Move.Len() > 0.1 {
			r.CornerChoice = OriginalDirectionIndex(in.Move) + 1
		} else {
			r.CornerChoice = 0
		}
	}
	if in.Move.Len() > 0.1 {
		p.Facing = in.Move.Unit()
	}
	pressed := in.Fire && !m.previous[r.Team].Fire
	if pressed {
		r.Armed = true
		r.ChargeTicks = 1
	}
	if r.Armed && in.Fire {
		r.ChargeTicks++
	}
	if r.Armed && (!in.Fire || r.ChargeTicks > TPS*2) {
		m.takeRestart(in.Move, r.ChargeTicks)
		return
	}
	if (r.Kind == ThrowIn || r.Kind == GoalKick) && pressed {
		m.takeRestart(in.Move, 5)
		return
	}
	if !m.Config.Human[r.Team] && r.Countdown <= 0 || r.Countdown <= -TPS*4 {
		charge := 6
		if r.Kind == Corner || r.Kind == FreeKick {
			charge = 20
		}
		m.takeRestart(Vec2{}, charge)
	}
}

func (m *Match) restartDirection(move Vec2) Vec2 {
	r := &m.Restart
	dir := float64(m.AttackDirection(r.Team))
	if move.Len() > 0.1 {
		if r.Kind == Penalty {
			return Vec2{move.X * 0.33, dir}.Unit()
		}
		if r.Kind == ThrowIn {
			inside := 1.0
			if r.Pos.X > PitchWidth/2 {
				inside = -1
			}
			return Vec2{math.Abs(move.X)*inside + inside*0.4, move.Y}.Unit()
		}
		return move.Unit()
	}
	if r.Kind == Penalty {
		return Vec2{m.PenaltyAim() * 0.30, dir}.Unit()
	}
	if r.Kind == Corner {
		target := Vec2{PitchWidth / 2, 70}
		if r.Pos.Y > PitchHeight/2 {
			target.Y = PitchHeight - 70
		}
		return target.Sub(r.Pos).Unit()
	}
	if r.Kind == ThrowIn || r.Kind == KickOff {
		index := m.passTarget(r.Team, r.Player)
		if index != r.Player {
			return m.Teams[r.Team].Players[index].Pos.Sub(r.Pos).Unit()
		}
	}
	if r.Kind == FreeKick {
		goal := Vec2{PitchWidth / 2, 0}
		if dir > 0 {
			goal.Y = PitchHeight
		}
		return goal.Sub(r.Pos).Unit()
	}
	return Vec2{(m.random() - 0.5) * 0.4, dir}.Unit()
}

func (m *Match) takeRestart(move Vec2, charge int) {
	r := m.Restart
	if r.Kind == Penalty {
		m.Restart.ChargeTicks = charge
		m.launchOriginalPenalty()
		return
	}
	if r.Kind == Corner {
		state := OriginalCornerState(r.Pos, r.CornerChoice, charge)
		m.Ball.applyOriginalState(state)
		m.Ball.TrappedBy = -1
		m.Ball.LastTeam, m.Ball.LastPlayer = r.Team, r.Player
		m.ballCurve = state.Curve
		m.aftertouchTicks = 0
		p := &m.Teams[r.Team].Players[r.Player]
		p.lastTouch = m.Tick
		p.touchCooldown = 15
		m.Phase = Playing
		m.emit("corner", r.Team, r.Player, r.Pos)
		m.MessageTicks = TPS
		return
	}
	power, lift := 300.0, 8.0
	kind := "pass"
	switch r.Kind {
	case ThrowIn:
		power = 220
		lift = 75
		kind = "throw"
		if move.Y*float64(m.AttackDirection(r.Team)) < 0 {
			power = 330
			lift = 110
		}
	case Corner:
		power = 260 + float64(charge)*3
		lift = 35 + float64(charge)*3
		kind = "corner"
	case GoalKick:
		power = 410
		lift = 120
		kind = "keeper_kick"
	case FreeKick:
		power = 420 + m.random()*60
		lift = float64(charge) * 3
		kind = "free_kick"
	}
	direction := m.restartDirection(move)
	p := &m.Teams[r.Team].Players[r.Player]
	p.Pos = r.Pos.Sub(direction.Mul(PlayerRadius + BallRadius + 2))
	p.Facing = direction
	m.kick(r.Team, r.Player, direction, power, clamp(lift, 0, 230), kind)
	m.Phase = Playing
	if r.Kind == KickOff {
		// The ordinary native kickoff uses EBE8's same action2 transition
		// as126F2; goal restarts already take their full source manager path.
		m.originalSetOfficialAction(2)
		m.originalClockRunning = true
	}
	m.MessageTicks = TPS
}

func (m *Match) penaltyTaker(team int) int {
	best := 1
	skill := -1.0
	for i := 1; i < 11; i++ {
		p := &m.Teams[team].Players[i]
		if !p.SentOff && p.Attributes.Shooting > skill {
			best = i
			skill = p.Attributes.Shooting
		}
	}
	return best
}

func (m *Match) arrangePenalty(team int) {
	direction := float64(m.AttackDirection(team))
	p := m.Restart.Player
	for t := range m.Teams {
		for i := range m.Teams[t].Players {
			q := &m.Teams[t].Players[i]
			q.Vel = Vec2{}
			if t == team && i == p {
				q.Pos = m.Restart.Pos.Sub(Vec2{0, direction * 18})
				q.Facing = Vec2{0, direction}
				continue
			}
			if t == 1-team && i == 0 {
				q.Pos = Vec2{PitchWidth / 2, 10}
				if direction > 0 {
					q.Pos.Y = PitchHeight - 10
				}
				continue
			}
			q.Pos = m.homePosition(t, i)
			if !q.Keeper {
				if direction < 0 && q.Pos.Y < 240 {
					q.Pos.Y = 240 + float64(i%3)*18
				}
				if direction > 0 && q.Pos.Y > PitchHeight-240 {
					q.Pos.Y = PitchHeight - 240 - float64(i%3)*18
				}
			}
		}
	}
}

// PenaltyAim returns the moving horizontal pointer used by a stationary penalty.
func (m *Match) PenaltyAim() float64 {
	if m.Restart.PenaltyAimStep == 0 {
		return 0
	}
	return float64(m.Restart.PenaltyAimX-456) / 50
}

// PracticeRestart puts the ball into a requested set piece without ending practice.
func (m *Match) PracticeRestart(kind RestartKind) {
	if !m.Config.Practice {
		return
	}
	pos := Vec2{PitchWidth / 2, 280}
	switch kind {
	case Penalty:
		pos.Y = PenaltySpotDepth
		if m.AttackDirection(0) > 0 {
			pos.Y = PitchHeight - PenaltySpotDepth
		}
	case Corner:
		pos = Vec2{5, 5}
		if m.AttackDirection(0) > 0 {
			pos.Y = PitchHeight - 5
		}
	case ThrowIn:
		pos = Vec2{5, PitchHeight / 2}
	case GoalKick:
		pos.Y = PitchHeight - 55
		if m.AttackDirection(0) > 0 {
			pos.Y = 55
		}
	case KickOff:
		pos.Y = PitchHeight / 2
	}
	m.beginRestart(kind, 0, pos, TPS)
}

// StartShootout enters the original 45/33/31/32/34/35 penalty-round flow.
// Standalone contests start at an expired final-period clock so the ordinary
// 11FD8 exit sequence runs when 136B4 clears the contest flag.
func (m *Match) StartShootout() {
	attackDownfield := m.AttackDirection(0) > 0
	m.clearBallControl()
	m.Config.Practice = false
	m.Phase, m.Half, m.Winner = Playing, 4, -1
	// Entering the final-period projection must not silently change ends.
	// 125D6 retains the current pitch orientation and runtime tactic tables.
	m.Config.StartDownfield = !attackDownfield
	p := &m.originalPeriod
	p.HalfFlag, p.ExtraStage, p.ExtraAllowed = 1, true, true
	p.EndStarted, p.Exit, p.Limit = false, false, m.originalClock
	m.originalClockRunning = true
	m.originalStartPenaltyContest()
}
