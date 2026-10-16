package engine

// originalPlayerStep retains only the local values between actor dispatch and
// its common movement/draw tail. It does not contain future reference states.
type originalPlayerStep struct {
	move              Vec2
	originalStep      int
	tacticDistance    float64
	controlled        bool
	previouslyVisible bool
	entryState        int
	ballWasVisible    bool
}

// originalCornerContinuation is the native counterpart of the blocked source
// call stack. Hosts may poll the selector without rerunning completed actors.
type originalCornerContinuation struct {
	active            bool
	team, actor       int
	closest, second   int
	step              originalPlayerStep
	cameraAfterActors bool
	cameraX, cameraY  int32
	cameraTarget      int
}

func (m *Match) prepareOriginalControls(input [2]Input) {
	for team := range input {
		if !m.Config.Human[team] {
			// Original CPU teams have no controlled-player or input record.
			input[team], m.cooperativeInputs[team] = Input{}, Input{}
			m.Teams[team].selectOwnPlayer(-1)
		}
		if !input[team].Fire {
			m.originalFireConsumed[team] = false
		}
		if !m.cooperativeInputs[team].Fire {
			m.originalFireConsumed[team+2] = false
		}
		if input[team].Move.Len() > 1 {
			input[team].Move = input[team].Move.Unit()
		}
		if input[team].TacticSlot != nil {
			m.QueueOriginalTacticSlot(team, *input[team].TacticSlot)
		}
	}
	m.controls = input
}

// completeOriginalPlayingUpdate can return at the blocking actor. On resume,
// updatePlayers finishes that actor and the remaining loop before this tail.
func (m *Match) completeOriginalPlayingUpdate(cameraAfterActors bool, cameraX, cameraY int32, cameraTarget int) bool {
	if !m.updatePlayers(m.controls) {
		pending := &m.originalCornerContinuation
		pending.cameraAfterActors, pending.cameraX, pending.cameraY, pending.cameraTarget = cameraAfterActors, cameraX, cameraY, cameraTarget
		m.originalUpdateCompleted = false
		return false
	}
	m.resolvePlayerCollisions()
	m.updateAftertouch(m.controls)
	m.updateOriginalScannerBall()
	if cameraAfterActors {
		// Preserve the entry target, but follow its completed movement. Freezing
		// the old coordinates adds a second delay to the next actor draw origin.
		if cameraTarget >= 0 {
			cameraX, cameraY = m.originalCameraFocusForTarget(cameraTarget)
		}
		m.updateOriginalCameraTo(cameraX, cameraY)
	}
	// 12560 increments the entry clock flag before dispatching rules.
	// A rule that starts the clock takes effect on the following update.
	if m.originalClockRunning {
		m.originalClock++
		m.ClockTicks++
	}
	m.publishOriginalHUD()
	if !m.Config.Practice {
		m.checkClock()
		if m.Phase != Playing {
			return true
		}
	}
	m.updateOriginalRules()
	if m.originalSubstitutionBlocked {
		m.originalUpdateCompleted = false
		return false
	}
	if m.originalKickoffReset {
		for side := range m.Teams {
			for index := range m.Teams[side].Players {
				p := &m.Teams[side].Players[index]
				if p.originalState != 0 {
					if p.originalState != 32 {
						p.originalState = 1
					}
					// 11D34 leaves inactive records and their history untouched.
					p.originalDirectionHistory = -1
				}
			}
		}
		m.originalKickoffReset = false
	}
	return true
}

func (m *Match) finishOriginalUpdate() {
	m.originalSubstitutionKeyboardPoll()
	if !m.originalPALHistorySampled {
		m.updateOriginalInputHistory()
	}
	m.previous, m.cooperativePrevious = m.controls, m.cooperativeInputs
}

func (m *Match) pollOriginalBlockedInput(input [2]Input, historySampled bool) {
	// The input interrupt ages the previous sample before accepting a new one.
	if !historySampled {
		m.updateOriginalInputHistory()
	}
	m.prepareOriginalControls(input)
}

func (m *Match) pollOriginalCorner(input [2]Input, historySampled bool) {
	m.pollOriginalBlockedInput(input, historySampled)
	team, actor := m.originalCornerTaker/11, m.originalCornerTaker%11
	in, _, _ := m.playerInput(team, actor)
	m.updateOriginalCornerSelector(in)
	if m.originalCornerSelecting {
		m.updateOriginalCamera()
		m.updateOriginalDisplayCamera()
		return
	}
	pending := m.originalCornerContinuation
	m.originalUpdateCompleted = true
	defer m.finishOriginalDisplayCamera()
	defer func() {
		if !m.originalCornerSelecting {
			m.finishOriginalVideo()
		}
	}()
	if m.completeOriginalPlayingUpdate(pending.cameraAfterActors, pending.cameraX, pending.cameraY, pending.cameraTarget) {
		m.finishOriginalUpdate()
	}
}
