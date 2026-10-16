package engine

// originalShotActor follows 10570..105E2, shared by both actor dispatch tables.
// Losing the current-player pointer ends aftertouch on a human team's shooter
// as well as on a CPU record. The partner-input wrapper also retains this
// primary selection check. AAB6 still receives the existing movement step;
// ordinary action 1 is not dispatched again in this same update.
func (m *Match) originalShotActor(team, index int) (Vec2, int) {
	p := &m.Teams[team].Players[index]
	if !m.Config.Aftertouch || !m.Teams[team].primaryControls(index) || p.touchCooldown <= 0 {
		p.originalState, p.originalDirectionHistory = 1, -1
		if m.aftertouchTeam == team && m.aftertouchPlayer == index && !m.aftertouchFreeKick {
			m.aftertouchTicks = 0
		}
		return p.Facing, p.originalStep
	}
	// 10584..105D0 applies input here, before the team's later selection pass.
	// Losing selection at that pass must not remove this actor's last impulse.
	in, _, _ := m.playerInput(team, index)
	if in.Move.Len() > .1 {
		state := OriginalShotAftertouchImpulse(originalBallState(m.Ball, m.ballCurve), m.aftertouchHeading, OriginalDirectionIndex(in.Move))
		m.Ball.applyOriginalState(state)
	}
	if m.aftertouchTeam == team && m.aftertouchPlayer == index && !m.aftertouchFreeKick {
		m.aftertouchTicks = max(0, p.touchCooldown-1)
	}
	return p.Facing, p.originalStep
}
