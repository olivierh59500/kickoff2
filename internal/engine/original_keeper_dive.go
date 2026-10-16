package engine

import "math"

func originalKeeperDiveAnimationStep(p *Player) {
	if int16(p.AnimationPhase) < 0 {
		return
	}
	old := p.AnimationPhase
	p.AnimationPhase += 170
	if old>>8 >= 20 {
		p.AnimationPhase = 65535
	}
}

// originalAnimatedKeeperContact follows13B7A. Its result precedes the caller's
// distinct caught/recovery state; failed saves retain the native parry draws.
func (m *Match) originalAnimatedKeeperContact(team int, p *Player) bool {
	m.Ball.LastTeam, m.Ball.LastPlayer = team, 0
	if p.touchCooldown > 0 || m.originalFoul.Active || m.originalGoalFrame.OutKind != 0 {
		return false
	}
	if int(OriginalRandomWord(&m.originalRandomState)%130) < int(math.Round(p.Attributes.Shooting*100)) {
		m.originalKeeperCollect(team, p)
		return true
	}
	m.originalKeeperParry(team, p)
	return false
}

// updateOriginalDiveKeeper translates1436A..1457C. Low dives decay their
// horizontal drift; high dives retain it until the shared ground recovery.
func (m *Match) updateOriginalDiveKeeper(team int, p *Player) bool {
	handled := true
	switch {
	case p.originalState == 47 || p.originalState == 49:
		if p.originalState == 47 {
			originalKeeperDrift(p)
		}
		if int16(p.AnimationPhase) >= 0 {
			if p.originalState == 49 {
				p.Pos.X += float64(p.originalKeeperDrift) / originalFixedScale
			}
			m.Ball.Pos = p.Pos
			originalKeeperDiveAnimationStep(p)
		} else {
			p.originalState, p.originalSubstate = 50, 0
			m.keeperHold = 100
		}
	case p.originalState == 50:
		originalKeeperDrift(p)
		m.keeperHold--
		if m.keeperHold <= 0 {
			p.originalState, p.originalSubstate = 44, 0
			p.originalDirectionHistory, p.AnimationPhase = -1, 0
			m.keeperHold = 50
		} else {
			originalKeeperDiveAnimationStep(p)
		}
	case p.originalState == 51:
		originalKeeperDrift(p)
		if p.originalBallDistance <= 144 && int(m.Ball.Height) <= 18 && m.originalAnimatedKeeperContact(team, p) {
			p.originalState, p.originalSubstate = 44, 0
			p.originalDirectionHistory = -1
			m.keeperHold = 100
		} else {
			m.keeperHold--
			if m.keeperHold <= 0 {
				p.originalState, p.originalSubstate, p.originalDirectionHistory = 1, 0, -1
			}
		}
	case p.originalSubstate == 45 || p.originalSubstate == 48:
		if int16(p.AnimationPhase) < 0 {
			p.originalSubstate = 46
			m.keeperHold = 25
		} else {
			if p.originalSubstate == 45 {
				originalKeeperDrift(p)
			} else {
				p.Pos.X += float64(p.originalKeeperDrift) / originalFixedScale
			}
			contact := p.originalBallDistance <= 144 && int(m.Ball.Height) <= 18
			caught := 47
			if p.originalSubstate == 48 {
				contact = p.originalBallDistance <= 144 && int(m.Ball.Height) >= 10 && int(m.Ball.Height) <= 32
				caught = 49
			}
			if contact && m.originalAnimatedKeeperContact(team, p) {
				p.originalState, p.originalSubstate, p.originalDirectionHistory = caught, 0, -1
			}
			originalKeeperDiveAnimationStep(p)
		}
	case p.originalSubstate == 46:
		originalKeeperDrift(p)
		if p.originalBallDistance <= 144 && int(m.Ball.Height) <= 8 && m.originalAnimatedKeeperContact(team, p) {
			p.originalState, p.originalSubstate, p.originalDirectionHistory = 50, 0, -1
			originalKeeperDiveAnimationStep(p)
		} else {
			m.keeperHold--
			if m.keeperHold <= 0 {
				p.originalSubstate, p.originalDirectionHistory, p.AnimationPhase = 0, -1, 0
			} else {
				originalKeeperDiveAnimationStep(p)
			}
		}
	default:
		handled = false
	}
	if handled {
		p.Vel = Vec2{}
		p.originalBallDistance = originalDistanceSquared(p.Pos, m.Ball.Pos)
	}
	return handled
}
