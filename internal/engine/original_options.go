package engine

// OriginalWind translates 0x6cda. Each option consumes three source random
// words, including OFF. The resulting signed 16.16 accelerations remain fixed
// for the match; OriginalBallStep applies them only above the source height gate.
func OriginalWind(option int, random *[3]uint16) (x, y int32) {
	strength := [...]int32{0, 500, 1000, 2000}[clampInt(option, 0, 3)]
	x = int32(OriginalRandomWord(random)%100+50) * strength
	y = int32(OriginalRandomWord(random)%100+50) * strength
	signs := OriginalRandomWord(random) % 4
	if signs&1 == 0 {
		x = -x
	}
	if signs&2 == 0 {
		y = -y
	}
	return x, y
}

// OriginalSpeedWait translates 0x793c..0x7946. The menu stores option indexes
// zero, one and two. The main loop at0xfc82 waits one base VBlank and then this
// many extra VBlanks, preserving complete integer steps for every actor/ball.
func OriginalSpeedWait(speed float64) int {
	option := 0
	if speed <= .25 {
		option = 2
	} else if speed <= .5 {
		option = 1
	}
	return (1 << option) >> 1
}

// UpdatePALControllers advances the 50 Hz host clock. UpdateControllers remains
// one complete source game-loop iteration for independent memory comparisons.
// Direction retention and the camera still receive video interrupts while a
// slower option waits; world movement and match timers advance as a unit.
func (m *Match) UpdatePALControllers(input [4]Input) bool {
	// Extra speed waits expose individual input samples between full updates.
	// 1660E/16666 retain the previous sample before polling the new one.
	if OriginalSpeedWait(m.Config.Speed) > 0 && !m.originalCornerSelecting && !m.originalSubstitutionBlocked {
		m.updateOriginalInputHistory()
		m.originalPALHistorySampled = true
		defer func() { m.originalPALHistorySampled = false }()
	}
	if m.originalPALWait == 0 || m.originalCornerSelecting || m.originalSubstitutionBlocked {
		m.UpdateControllers(input)
		m.originalPALWait = OriginalSpeedWait(m.Config.Speed)
		return m.originalUpdateCompleted
	}
	m.originalPALWait--
	m.Events = m.Events[:0]
	for slot := 2; slot < len(input); slot++ {
		control := input[slot]
		team := slot % 2
		if control.TacticSlot != nil && m.Config.Human[team] && (slot < 2 || m.Config.Cooperative[team]) {
			m.QueueOriginalTacticSlot(team, *control.TacticSlot)
		}
	}
	m.cooperativeInputs = [2]Input{input[2], input[3]}
	// A release clears the source fire-consumed word even between world steps.
	m.prepareOriginalControls([2]Input{input[0], input[1]})
	m.updateOriginalCamera()
	m.updateOriginalDisplayCamera()
	return false
}
