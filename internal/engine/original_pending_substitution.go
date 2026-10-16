package engine

import "math"

// OriginalPendingSubstitutionState models19D5A/19D5E/19D62 and the player
// view pointer. Positive1/2 reserves a bench choice;11/12 selects the outgoing
// player. The transient negative and20+ values belong to the main-loop notice.
type OriginalPendingSubstitutionState struct {
	Word    int16
	Team    int
	Cursor  int
	Viewing bool
}

func originalPendingReserve(word int16) int {
	value := int(word)
	if value < 0 {
		value = -value
	}
	if value >= 20 {
		value -= 20
	} else if value >= 10 {
		value -= 10
	}
	if value == 1 || value == 2 {
		return value - 1
	}
	return -1
}

func (m *Match) OriginalPendingSubstitution() OriginalPendingSubstitutionState {
	return m.originalPendingSubstitution
}

// OriginalSubstitutionDisplay retains the HUD prepared before the keyboard
// poll writes the next drawing buffer's cursor text.
func (m *Match) OriginalSubstitutionDisplay() OriginalPendingSubstitutionState {
	return m.originalSubstitutionDisplay
}

func (m *Match) OriginalSubstitutionSelection() (team, player int, selecting bool) {
	p := m.originalPendingSubstitution
	if p.Word < 10 || p.Word >= 20 || m.originalRuleState != 40 {
		return 0, 0, false
	}
	return p.Cursor / 11, p.Cursor % 11, p.Cursor >= 0 && p.Cursor < 22
}

// QueueOriginalSubstitutionKey supplies an ordinary decoded keyboard event.
// The original polls these keys after the world/rule update, not before it.
func (m *Match) QueueOriginalSubstitutionKey(key int) bool {
	switch key {
	case 0x3e, 0x3f, 0x43, 0x44, 0x48, 0x50, 0x4b, 0x4d:
		return m.QueueOriginalKeyboardEvent(uint32(key) << 16)
	case 0x20, 0x66:
		return m.QueueOriginalKeyboardEvent(uint32(key))
	default:
		return false
	}
}

// QueueOriginalKeyboardEvent retains16824's packed scan/character word. The
// shared post-world poll preserves ordering between tactics and substitutions.
func (m *Match) QueueOriginalKeyboardEvent(key uint32) bool {
	character, scan := key&65535, key>>16
	if character < 0x31 || character > 0x34 {
		if character != 0x20 && character != 0x66 && character != 0x63 {
			switch scan {
			case 0x3e, 0x3f, 0x43, 0x44, 0x48, 0x50, 0x4b, 0x4d:
			default:
				return false
			}
		}
	}
	if m.Phase == FullTime || m.originalSubstitutionBlocked {
		return false
	}
	m.originalKeyboardEvents = append(m.originalKeyboardEvents, key)
	return true
}

// RequestOriginalSubstitution is FE60..FE9A's already-polled key operation.
// The source writes the team pointer before rejecting an occupied request or
// CPU team; retaining this side effect matters when both humans press F-keys.
func (m *Match) RequestOriginalSubstitution(team, reserve int) bool {
	if team < 0 || team > 1 || reserve < 0 || reserve > 1 || m.Phase != Playing {
		return false
	}
	p := &m.originalPendingSubstitution
	p.Team = team
	if !m.Config.Human[team] || p.Word != 0 || m.Teams[team].BenchUsed[reserve] {
		return false
	}
	m.Teams[team].BenchUsed[reserve] = true
	p.Word = int16(reserve + 1)
	return true
}

func (m *Match) originalEligibleSubstitutionActor(team, player int) bool {
	if team < 0 || team > 1 || player < 0 || player > 10 {
		return false
	}
	p := &m.Teams[team].Players[player]
	return p.originalInjuryWord == 0 && p.originalBookings != 2
}

// SelectOriginalSubstitutionPlayer supplies the same player choice as the
// source cursor. The keeper is not silently excluded: originalFF96 includes0.
func (m *Match) SelectOriginalSubstitutionPlayer(team, player int) bool {
	if _, _, selecting := m.OriginalSubstitutionSelection(); !selecting || m.originalPendingSubstitution.Cursor/11 != team || !m.originalEligibleSubstitutionActor(team, player) {
		return false
	}
	m.originalPendingSubstitution.Cursor = team*11 + player
	return true
}

func (m *Match) ConfirmOriginalSubstitutionPlayer() bool {
	if _, _, selecting := m.OriginalSubstitutionSelection(); !selecting {
		return false
	}
	m.originalPendingSubstitution.Word = 0
	m.originalPendingSubstitution.Viewing = false
	return true
}

func (m *Match) originalMoveSubstitutionCursor(delta int) {
	p := &m.originalPendingSubstitution
	if p.Cursor < 0 || p.Cursor >= 22 {
		return
	}
	team := p.Cursor / 11
	shirt := m.Teams[team].Players[p.Cursor%11].Number - 1
	for attempt := 0; attempt < 28; attempt++ {
		shirt = (shirt + delta + 14) % 14
		for index := range m.Teams[team].Players {
			if m.Teams[team].Players[index].Number-1 == shirt && m.originalEligibleSubstitutionActor(team, index) {
				p.Cursor = team*11 + index
				return
			}
		}
	}
}

func (m *Match) originalOpenPlayerView() {
	p := &m.originalPendingSubstitution
	if p.Viewing {
		return
	}
	if p.Cursor < 0 || p.Cursor >= 22 {
		p.Cursor = 0
		if m.Ball.LastTeam >= 0 && m.Ball.LastPlayer >= 0 {
			p.Cursor = m.Ball.LastTeam*11 + m.Ball.LastPlayer
		}
	}
	p.Viewing = true
}

func (m *Match) originalViewSubstitutionTeam(opposite bool) {
	p := &m.originalPendingSubstitution
	if p.Cursor < 0 || p.Cursor >= 22 {
		return
	}
	shirt := m.Teams[p.Cursor/11].Players[p.Cursor%11].Number - 1
	team := p.Team
	if opposite {
		team = 1 - team
	}
	for attempt := 0; attempt < 14; attempt++ {
		for index := range m.Teams[team].Players {
			if m.Teams[team].Players[index].Number-1 == shirt && m.originalEligibleSubstitutionActor(team, index) {
				p.Cursor = team*11 + index
				return
			}
		}
		shirt = (shirt + 1) % 14
	}
}

// originalSubstitutionNotice follows FCF2..FD26 before the keyboard poll.
func (m *Match) originalSubstitutionNotice() {
	p := &m.originalPendingSubstitution
	if p.Word < 0 {
		p.Word = -p.Word + 10
		m.originalOpenPlayerView()
		m.originalViewSubstitutionTeam(false)
	} else if p.Word >= 20 {
		p.Word = 0
	}
}

func (m *Match) originalApplySubstitutionKey(key int) {
	switch key {
	case 0x3e, 0x3f:
		m.RequestOriginalSubstitution(0, key-0x3e)
	case 0x43, 0x44:
		m.RequestOriginalSubstitution(1, key-0x43)
	case 0x20:
		m.ConfirmOriginalSubstitutionPlayer()
	case 0x66:
		if m.originalPendingSubstitution.Word < 10 {
			if m.originalPendingSubstitution.Viewing {
				m.originalPendingSubstitution.Viewing = false
				// FEBC/FEC4 restore the ball camera after closing the player view.
				m.originalCameraTarget = -1
			} else {
				m.originalOpenPlayerView()
			}
		}
	case 0x48:
		if m.originalPendingSubstitution.Viewing {
			m.originalMoveSubstitutionCursor(1)
		}
	case 0x50:
		if m.originalPendingSubstitution.Viewing {
			m.originalMoveSubstitutionCursor(-1)
		}
	case 0x4b:
		if m.originalPendingSubstitution.Viewing && m.originalPendingSubstitution.Word < 10 {
			m.originalViewSubstitutionTeam(false)
		}
	case 0x4d:
		if m.originalPendingSubstitution.Viewing && m.originalPendingSubstitution.Word < 10 {
			m.originalViewSubstitutionTeam(true)
		}
	}
}

func (m *Match) originalSubstitutionKeyboardPoll() {
	m.originalSubstitutionDisplay = m.originalPendingSubstitution
	m.originalSubstitutionNotice()
	for _, key := range m.originalKeyboardEvents {
		if key&65535 == 0x63 {
			// FD98/FD9E run after the current world and its sound requests.
			m.Config.CrowdMuted = !m.Config.CrowdMuted
			continue
		}
		if m.originalApplyTacticKey(key) {
			continue
		}
		if character := key & 65535; character == 0x20 || character == 0x66 {
			m.originalApplySubstitutionKey(int(character))
		} else {
			m.originalApplySubstitutionKey(int(key >> 16))
		}
	}
	m.originalKeyboardEvents = m.originalKeyboardEvents[:0]
}

func (m *Match) originalFindReserveRecord(team, reserve int) (int, bool) {
	shirt := 12 + reserve*2
	for index := range m.Teams[team].Players {
		if m.Teams[team].Players[index].Number == shirt {
			return index, true
		}
	}
	for index := range m.Teams[team].Bench {
		if m.Teams[team].Bench[index].Number == shirt {
			return 11 + index, true
		}
	}
	return 0, false
}

// originalBeginPendingSubstitution translates135D2. Callers establish the
// exact rule/wait to resume before freezing the active actors under action54.
func (m *Match) originalBeginPendingSubstitution() bool {
	p := &m.originalPendingSubstitution
	if p.Word <= 0 || p.Word > 2 {
		return false
	}
	reserve := int(p.Word) - 1
	record, found := m.originalFindReserveRecord(p.Team, reserve)
	if !found {
		return false
	}
	sub := &m.originalSubstitution
	sub.Reserve = reserve
	sub.ReserveTeam, sub.HasReserveTeam, sub.ReserveRecord, sub.HasReserveRecord = p.Team, true, record, true
	sub.ResumeRule, sub.ResumeWait = m.originalRuleState, m.originalRuleWait
	p.Word = -p.Word
	m.originalAssignActiveState(54)
	m.originalRuleState, m.originalClockRunning = 40, false
	m.notice("SUBSTITUTE PLAYER", 30000)
	return true
}

// originalStartSelectedSubstitution is1339A: confirmation only starts the
// departure wait. Identity does not change until41 reaches the touchline.
func (m *Match) originalStartSelectedSubstitution() {
	p, sub := &m.originalPendingSubstitution, &m.originalSubstitution
	if p.Word != 0 || p.Cursor < 0 || p.Cursor >= 22 {
		return
	}
	sub.Team, sub.Player = p.Cursor/11, p.Cursor%11
	actor := &m.Teams[sub.Team].Players[sub.Player]
	sub.ReturnTarget = Vec2{math.Floor(actor.Pos.X), math.Floor(actor.Pos.Y)}
	m.originalCameraHoldX = int32(sub.ReturnTarget.X)<<16 | int32(uint16(m.originalCameraHoldX))
	m.originalCameraHoldY = int32(sub.ReturnTarget.Y)<<16 | int32(uint16(m.originalCameraHoldY))
	m.originalRuleState, m.originalRuleWait = 41, 50
}

func (m *Match) originalCancelPendingSubstitution() {
	p := &m.originalPendingSubstitution
	if p.Word == 0 || p.Word >= 20 {
		return
	}
	if reserve := originalPendingReserve(p.Word); reserve >= 0 {
		m.Teams[p.Team].BenchUsed[reserve] = false
	}
	p.Word += 20
}
