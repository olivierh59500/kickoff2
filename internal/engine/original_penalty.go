package engine

import "math"

// OriginalPenaltyAimStep translates the visible aiming marker at0xe8b2..0xe930.
// It moves three source pixels per update and reverses beyond506 or at405.
func OriginalPenaltyAimStep(x, step int) (int, int) {
	x += step
	if x > 506 || x <= 406 {
		step = -step
	}
	return x, step
}

// OriginalPenaltyState translates the human launch at0xe848..0xe878. The marker
// is fifty units ahead of the spot; charge is the actual held-fire update count.
// Four isolated original CPU transactions independently verify these launch
// words. Complete penalty-phase parity still requires natural phase captures.
func OriginalPenaltyState(position Vec2, aimX, charge int) OriginalBallState {
	x, y := int(math.Floor(position.X)), int(math.Floor(position.Y))
	targetY := y - 50
	if y > 696 {
		targetY = y + 50
	}
	return OriginalBallState{X: int32(math.Round(position.X * originalFixedScale)), Y: int32(math.Round(position.Y * originalFixedScale)), VelocityX: int32(int16(aimX-x)) * 12000, VelocityY: int32(int16(targetY-y)) * 12000, Lift: int32(uint16(charge)) * 20000}
}

// OriginalComputerPenaltyState translates C5A2..C65E. The computer uses one
// random lateral error and six alternating lift errors, rather than the human
// moving marker and held-button charge. Words must come from the source RNG.
func OriginalComputerPenaltyState(position Vec2, skill, attack int, words [7]uint16) OriginalBallState {
	skill = clampInt(skill, 0, 100)
	errorRange := 400 - skill
	lateral := (int(words[0])%errorRange - (errorRange >> 1)) * 1400
	lift := 108
	for i, word := range words[1:] {
		error := int(word) % (101 - skill)
		if i&1 == 0 {
			lift += error
		} else {
			lift -= error
		}
	}
	return OriginalBallState{X: int32(math.Round(position.X * originalFixedScale)), Y: int32(math.Round(position.Y * originalFixedScale)), VelocityX: int32(lateral), VelocityY: int32(attack * 500000), Lift: int32(lift * 1000)}
}

func (m *Match) updateOriginalPenaltyRestart(input [2]Input) {
	restart := &m.Restart
	defender := 1 - restart.Team
	if m.Config.Human[defender] {
		m.Teams[defender].selectOwnPlayer(0)
		m.updatePenaltyKeeper(defender, &m.Teams[defender].Players[0], input[defender])
	}
	restart.Countdown--
	in := input[restart.Team]
	human := m.Config.Human[restart.Team]
	if !human && restart.Countdown <= 0 {
		restart.Armed = true
	}
	if in.Fire && !m.previous[restart.Team].Fire {
		restart.Armed = true
		restart.ChargeTicks = 1
	}
	player := &m.Teams[restart.Team].Players[restart.Player]
	player.originalStep = 0
	if restart.Armed {
		if originalDistanceSquared(player.Pos, restart.Pos) <= 16 {
			m.launchOriginalPenalty()
			return
		}
		player.Facing = OriginalDirection(0)
		if m.AttackDirection(restart.Team) > 0 {
			player.Facing = OriginalDirection(4)
		}
		player.originalStep = 150
		if !human {
			player.originalStep = originalPace(player)
		}
		player.Vel = player.Facing.Mul(float64(player.originalStep) * 1000 * TPS / originalFixedScale)
		player.Pos = player.Pos.Add(player.Vel.Mul(1.0 / TPS))
		if in.Fire {
			restart.ChargeTicks++
		}
	}
	advanceOriginalAnimation(player)
	if !human {
		restart.PenaltyAimStep = 0
		return
	}
	if restart.PenaltyAimStep == 0 {
		restart.PenaltyAimX, restart.PenaltyAimStep = 456, 3
	}
	restart.PenaltyAimX, restart.PenaltyAimStep = OriginalPenaltyAimStep(restart.PenaltyAimX, restart.PenaltyAimStep)
}
func (m *Match) launchOriginalPenalty() {
	restart := m.Restart
	player := &m.Teams[restart.Team].Players[restart.Player]
	state := OriginalPenaltyState(m.Ball.Pos, restart.PenaltyAimX, restart.ChargeTicks)
	if !m.Config.Human[restart.Team] {
		var words [7]uint16
		for i := range words {
			words[i] = OriginalRandomWord(&m.originalRandomState)
		}
		state = OriginalComputerPenaltyState(m.Ball.Pos, int(math.Round(player.Attributes.Shooting*100)), m.AttackDirection(restart.Team), words)
	}
	m.Ball.applyOriginalState(state)
	m.ballCurve = 0
	m.Ball.TrappedBy = -1
	m.originalBallOwner = -1
	m.Ball.LastTeam, m.Ball.LastPlayer = restart.Team, restart.Player
	m.originalLastShooter = restart.Team*11 + restart.Player
	player.touchCooldown, player.lastTouch = 15, m.Tick
	player.originalState = 1
	// The source forces step150 before the contact check, then jumps to AAB6
	// after launching. Its final approach step still occurs on the launch update.
	player.originalStep = 150
	if !m.Config.Human[restart.Team] {
		player.originalStep, player.originalState = 0, 30
	}
	player.Vel = player.Facing.Mul(float64(player.originalStep) * 1000 * TPS / originalFixedScale)
	player.Pos = player.Pos.Add(player.Vel.Mul(1.0 / TPS))
	advanceOriginalAnimation(player)
	m.aftertouchTicks = 0
	m.aftertouchLateral, m.aftertouchVertical = 0, 0
	m.penaltyFlightTicks, m.penaltyTeam = TPS*3, restart.Team
	m.Restart.PenaltyAimStep = 0
	m.Restart.ChargeTicks = 0
	m.Phase = Playing
	m.notice("PENALTY", TPS)
	m.emit("penalty", restart.Team, restart.Player, m.Ball.Pos)
}
