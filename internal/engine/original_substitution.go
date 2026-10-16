package engine

import "math"

// OriginalSubstitutionState retains1364C's saved rule, outgoing actor and return
// position. Reserve indexes0/1 identify stored source shirt words11/13.
type OriginalSubstitutionState struct {
	ReserveTeam, ReserveRecord       int
	HasReserveTeam, HasReserveRecord bool
	Team, Player, Reserve            int
	ResumeRule, ResumeWait           int
	ReturnTarget                     Vec2
	ExitY                            int
}

// OriginalInjurySubstitutePrompt exposes the visible original reserve selector.
// F4/F5 and Android touch choices enter through QueueOriginalInjuryReserve.
func (m *Match) OriginalInjurySubstitutePrompt() (team int, pending bool) {
	return m.originalSubstitution.Team, m.originalRuleState == 44 || m.originalSubstitutionBlocked
}

func (m *Match) QueueOriginalInjuryReserve(reserve int) bool {
	team, pending := m.OriginalInjurySubstitutePrompt()
	if !pending || team < 0 || team >= 2 || reserve < 0 || reserve >= 2 || m.Teams[team].BenchUsed[reserve] {
		return false
	}
	m.originalSubstituteChoice, m.originalHasSubstituteChoice = reserve, true
	return true
}

func originalCoordinateWord(previous float64, integer int) float64 {
	return float64(integer) + previous - math.Floor(previous)
}

// QueueOriginalInjuryReserveInputs also supplies the actual joystick state
// delivered while the original keyboard selector blocks. Prediction uses the
// external UI journal, never the observed replacement's attributes or position.
func (m *Match) QueueOriginalInjuryReserveInputs(reserve int, inputs [4]Input) bool {
	if !m.QueueOriginalInjuryReserve(reserve) {
		return false
	}
	m.originalSubstituteInputs, m.originalHasSubstituteInputs = inputs, true
	return true
}

// QueueOriginalInjuryReservePolls supplies measured external input samples from
// a blocking reserve screen. It is used by original comparison tools; the live
// host supplies one sample per ordinary UpdateControllers call instead.
func (m *Match) QueueOriginalInjuryReservePolls(reserve int, inputs [4]Input, polls [][4]Input) bool {
	if len(polls) > 128 || !m.QueueOriginalInjuryReserveInputs(reserve, inputs) {
		return false
	}
	m.originalSubstitutePolls = append(m.originalSubstitutePolls[:0], polls...)
	return true
}

// beginOriginalInjurySubstitution follows128DE..129E8. Human teams wait for
// the original keyboard selector; CPU teams choose the first unused reserve.
// An exhausted bench instead sends the injured actor out under rule38.
func (m *Match) beginOriginalInjurySubstitution() {
	m.originalCancelPendingSubstitution()
	foul := &m.originalFoul
	p := &m.Teams[foul.Team].Players[foul.Victim%11]
	p.originalState, p.originalDirectionHistory = 4, -1
	previous := m.originalSubstitution
	m.originalSubstitution = OriginalSubstitutionState{Team: foul.Team, Player: foul.Victim % 11, Reserve: -1,
		ReserveTeam: previous.ReserveTeam, HasReserveTeam: previous.HasReserveTeam, ReserveRecord: previous.ReserveRecord, HasReserveRecord: previous.HasReserveRecord,
		ResumeRule: 39, ResumeWait: 150, ReturnTarget: Vec2{math.Floor(p.Pos.X), math.Floor(p.Pos.Y)}, ExitY: previous.ExitY}
	m.notice(p.Name+" INJURED!", 300)
	m.originalCameraHoldX = int32(math.Floor(p.Pos.X))<<16 | int32(uint16(m.originalCameraHoldX))
	m.originalCameraHoldY = int32(math.Floor(p.Pos.Y))<<16 | int32(uint16(m.originalCameraHoldY))
	reserve := -1
	for slot, used := range m.Teams[foul.Team].BenchUsed {
		if !used {
			reserve = slot
			break
		}
	}
	if reserve < 0 {
		m.originalRuleState = 38
		return
	}
	m.originalClockRunning = false
	if m.Config.Human[foul.Team] {
		m.originalRuleState, m.originalRuleWait = 43, 150
		m.MessageTicks = 75
		return
	}
	m.originalChooseInjuryReserve(reserve)
}

func (m *Match) originalChooseInjuryReserve(reserve int) {
	sub := &m.originalSubstitution
	sub.Reserve = reserve
	if record, found := m.originalFindReserveRecord(sub.Team, reserve); found {
		sub.ReserveTeam, sub.HasReserveTeam, sub.ReserveRecord, sub.HasReserveRecord = sub.Team, true, record, true
	}
	m.Teams[sub.Team].BenchUsed[reserve] = true
	m.originalRuleState, m.originalRuleWait = 41, 50
	m.originalHasSubstituteChoice, m.originalSubstitutionBlocked = false, false
	if m.originalHasSubstituteInputs {
		if len(m.originalSubstitutePolls) != 0 {
			m.originalPALHistorySampled = false
		}
		for _, inputs := range m.originalSubstitutePolls {
			m.updateOriginalInputHistory()
			m.prepareOriginalControls([2]Input{inputs[0], inputs[1]})
			m.cooperativeInputs = [2]Input{inputs[2], inputs[3]}
			// Each external sample corresponds to one ordinary host wait:
			// presentation continues while football state remains frozen.
			m.updateOriginalDisplayCamera()
			m.updateOriginalCamera()
		}
		if len(m.originalSubstitutePolls) != 0 {
			m.updateOriginalInputHistory()
			m.updateOriginalDisplayCamera()
			m.updateOriginalCamera()
		}
		m.controls = [2]Input{m.originalSubstituteInputs[0], m.originalSubstituteInputs[1]}
		m.cooperativeInputs = [2]Input{m.originalSubstituteInputs[2], m.originalSubstituteInputs[3]}
		m.originalHasSubstituteInputs = false
	}
}

// updateOriginalDepartingPlayer follows DE5E: target(-400,680/712), full pace
// from C736, and the ordinary runner animation. The manager removes the actor
// once its integer X reaches the touchline threshold.
func (m *Match) updateOriginalDepartingPlayer(team int, p *Player) (Vec2, int) {
	y := 680.0
	if m.AttackDirection(team) > 0 {
		y = 712
	}
	move, step, _ := m.originalRunToTarget(p, Vec2{-400, y})
	return move, step
}

func (m *Match) originalRemoveDepartingPlayer(p *Player) {
	p.Pos.X, p.Pos.Y = originalCoordinateWord(p.Pos.X, -5000), originalCoordinateWord(p.Pos.Y, -5000)
	p.originalState, p.originalStep, p.Vel = 0, 0, Vec2{}
	p.originalVisible = false // F006 removes the departing actor's draw record.
}

// originalSwapInReserve translates133CC/13416's three word ranges. Position,
// heading, animation, cached ball distance and the field's tactics stay attached
// to the playing record while shirt, name, skill and discipline move from bench.
func (m *Match) originalSwapInReserve() {
	sub := &m.originalSubstitution
	reserveTeam := sub.Team
	if sub.HasReserveTeam {
		reserveTeam = sub.ReserveTeam
	}
	reserveRecord := 11 + sub.Reserve
	if sub.HasReserveRecord {
		reserveRecord = sub.ReserveRecord
	}
	var reserve *Player
	if reserveRecord < 11 {
		reserve = &m.Teams[reserveTeam].Players[reserveRecord]
	} else {
		reserve = &m.Teams[reserveTeam].Bench[reserveRecord-11]
	}
	p := &m.Teams[sub.Team].Players[sub.Player]
	if !p.Keeper {
		p.Number, reserve.Number = reserve.Number, p.Number
	}
	p.Name, reserve.Name = reserve.Name, p.Name
	p.Attributes, reserve.Attributes = reserve.Attributes, p.Attributes
	p.OriginalID, reserve.OriginalID = reserve.OriginalID, p.OriginalID
	p.OriginalRecord, reserve.OriginalRecord = reserve.OriginalRecord, p.OriginalRecord
	p.originalPaceFixed, reserve.originalPaceFixed = reserve.originalPaceFixed, p.originalPaceFixed
	p.originalStaminaWord, reserve.originalStaminaWord = reserve.originalStaminaWord, p.originalStaminaWord
	p.originalAggressionThreshold, reserve.originalAggressionThreshold = reserve.originalAggressionThreshold, p.originalAggressionThreshold
	p.originalFoulPoints, reserve.originalFoulPoints = reserve.originalFoulPoints, p.originalFoulPoints
	p.originalBookings, reserve.originalBookings = reserve.originalBookings, p.originalBookings
	p.originalRecoveryLimit, reserve.originalRecoveryLimit = reserve.originalRecoveryLimit, p.originalRecoveryLimit
	p.originalOpaqueWords, reserve.originalOpaqueWords = reserve.originalOpaqueWords, p.originalOpaqueWords
	p.originalOpaqueWords[2] = 1 // 13468 marks the incoming field record at+34.
	p.originalSubstate, reserve.originalSubstate = reserve.originalSubstate, p.originalSubstate
	p.originalGoalCount, reserve.originalGoalCount = reserve.originalGoalCount, p.originalGoalCount
	p.originalInjuryWord, reserve.originalInjuryWord = reserve.originalInjuryWord, p.originalInjuryWord
	p.originalState, p.originalDirectionHistory = 55, -1
	p.originalVisible = false // 13442 clears the field record's retained draw pointer.
	p.Pos.X, p.Pos.Y = originalCoordinateWord(p.Pos.X, -40), originalCoordinateWord(p.Pos.Y, sub.ExitY)
	p.originalRestartTarget = sub.ReturnTarget
	m.originalCameraTarget = sub.Team*11 + sub.Player
	// 13472 compares the retained reserve B4 with both primary headers in
	// order. The selected actor may belong to the opposing team. The binding
	// stays on the reserve slot; selection does not transfer physical input.
	if !m.originalControlHandoffSuppressed && m.Config.Human[reserveTeam] {
		role := uint8(0)
		if m.Config.HasSquadBindings[reserveTeam] {
			role = m.Config.SquadBindings[reserveTeam][reserveRecord]
		}
		owner := -1
		if role == 1 {
			owner = reserveTeam
		} else if role == 3 && m.Config.Human[1-reserveTeam] {
			owner = 1 - reserveTeam
		}
		if owner >= 0 {
			m.selectControlledActor(owner, sub.Team*11+sub.Player)
		}
	}
	m.originalRuleState = 42
	m.Teams[reserveTeam].Substitutions++
	m.notice("SUB- "+p.Name, 250)
	m.emit("substitution", sub.Team, sub.Player, p.Pos)
}

func (m *Match) updateOriginalInjurySubstitution() {
	sub := &m.originalSubstitution
	if m.originalRuleState == 40 {
		m.originalStartSelectedSubstitution()
		return
	}
	p := &m.Teams[sub.Team].Players[sub.Player]
	switch m.originalRuleState {
	case 38:
		if int(math.Floor(p.Pos.X)) >= 140 {
			m.originalCameraHoldX = int32(math.Round(p.Pos.X * originalFixedScale))
			m.originalCameraHoldY = int32(math.Round(p.Pos.Y * originalFixedScale))
		}
		m.originalCameraTarget = -3
		if int(math.Floor(p.Pos.X)) <= -60 {
			m.originalRemoveDepartingPlayer(p)
			if next := m.originalNearest(sub.Team, -1); next >= 0 {
				m.originalFoul.Victim, sub.Player = sub.Team*11+next, next
				m.originalCameraTarget = m.originalFoul.Victim
				m.originalRuleState, m.originalRuleWait = 39, 150
			}
		}
	case 41:
		m.originalRuleWait--
		if m.originalRuleWait > 0 {
			return
		}
		if m.originalRuleWait == 0 {
			p.originalState, p.originalDirectionHistory = 4, -1
		}
		if int(math.Floor(p.Pos.X)) >= 0 {
			m.originalCameraHoldX = int32(max(140, int(math.Floor(p.Pos.X))))<<16 | int32(uint16(m.originalCameraHoldX))
			m.originalCameraHoldY = int32(math.Floor(p.Pos.Y))<<16 | int32(uint16(m.originalCameraHoldY))
		}
		m.originalCameraTarget = -3
		if int(math.Floor(p.Pos.X)) <= -40 {
			m.originalRemoveDepartingPlayer(p)
		}
		if p.originalState != 0 {
			sub.ExitY = int(math.Floor(p.Pos.Y))
			return
		}
		m.originalSwapInReserve()
	case 42:
		if int(math.Floor(p.Pos.X)) >= -30 && p.originalStep == 0 {
			m.originalRuleState, m.originalRuleWait = sub.ResumeRule, sub.ResumeWait
			m.originalClockRunning = true
			m.MessageTicks = 10
			m.originalCameraTarget = -1
		}
	case 43:
		if m.MessageTicks <= 0 {
			m.originalRuleState = 44
			m.notice("SELECT SUBSTITUTE", 25)
		}
	case 39:
		m.originalRuleWait--
		if m.originalRuleWait <= 0 {
			m.originalDispatchFoulRestart()
		}
	case 44:
		m.originalCancelPendingSubstitution()
		if m.originalHasSubstituteChoice {
			m.originalChooseInjuryReserve(m.originalSubstituteChoice)
		} else {
			m.originalSubstitutionBlocked = true
		}
	}
}
