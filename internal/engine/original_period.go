package engine

// OriginalPeriodState retains the source clock limits, exit latch and penalty
// round counters. ClockOffset is a native cumulative-time projection; the
// actual source clock resets to zero when extra time begins atA078.
type OriginalPeriodState struct {
	Limit, FullLimit, RegulationFull, ClockOffset uint32
	HalfFlag                                      int
	EndStarted, Exit                              bool
	ExtraAllowed, ExtraStage, PenaltyContest      bool
	InitialKickoff, Attempt                       int
	ShooterNumbers                                [2]int
	ContestInitialScores                          [2]int
}

func (m *Match) OriginalPeriod() OriginalPeriodState { return m.originalPeriod }

// originalEndPeriod is11FD8. Actors keep moving in the live match loop; the
// first rule12 decrement is performed by the same125FA dispatch as the award.
func (m *Match) originalEndPeriod() {
	period := &m.originalPeriod
	if period.EndStarted {
		return
	}
	period.EndStarted = true
	m.originalAssignActiveState(4)
	m.originalRuleState, m.originalRuleWait, m.originalClockRunning = 12, 100, false
	message := "HALF TIME"
	if period.HalfFlag != 0 {
		message = "FULL TIME"
	}
	m.notice(message, 10000)
	m.emit("period_end", -1, -1, m.Ball.Pos)
}

func (m *Match) originalCheckPeriodClock() {
	period := &m.originalPeriod
	if m.originalClock < period.Limit {
		return
	}
	if m.originalFoul.StoppageClock != 0 {
		m.originalClock = period.Limit
		m.ClockTicks = int(period.ClockOffset + period.Limit)
		return
	}
	if lost := m.originalFoul.LostTicks; lost != 0 {
		period.Limit += lost
		period.FullLimit += lost
		m.originalAppliedInjuryTicks += lost
		m.originalPeriodInjuryTicks += lost
		m.originalFoul.LostTicks = 0
		// 125A8->F818 replaces the persistent caption without disturbing the
		// current temporary notice or its countdown. It remains underneath it.
		m.originalPersistentMessage = "INJURY TIME"
		return
	}
	if period.PenaltyContest {
		return
	}
	if period.HalfFlag != 0 && period.ExtraStage && period.ExtraAllowed && m.Teams[0].Score == m.Teams[1].Score {
		m.originalStartPenaltyContest()
		return
	}
	m.originalEndPeriod()
}

func (m *Match) updateOriginalPeriodRule() {
	switch m.originalRuleState {
	case 12:
		m.originalRuleWait--
		if m.originalRuleWait <= 0 {
			m.originalCameraFixedX, m.originalCameraFixedY = 160<<16, 696<<16
			m.originalCameraTarget = -2
			m.originalRuleState, m.originalRuleWait = 13, 100
		}
	case 13:
		m.originalRuleWait--
		if m.originalRuleWait <= 0 {
			m.originalAssignActiveState(4)
			m.originalRuleState, m.originalRuleWait = 14, 100
		}
	case 14:
		if m.originalRuleWait == 20 {
			m.originalCancelPendingSubstitution()
		}
		m.originalRuleWait--
		if m.originalRuleWait <= 0 {
			m.originalPeriod.Exit = true
			m.originalCompletePeriod()
		}
	case 35:
		m.originalRuleWait--
		if m.originalRuleWait <= 0 {
			if m.originalGoalFrame.OutKind >= 3 {
				team := int(m.originalGoalFrame.OutKind) - 3
				m.Teams[team].Score++
				actor := m.originalFoul.Victim
				m.Teams[actor/11].Players[actor%11].originalGoalCount++
				m.ShootoutScore[team]++
				m.emit("goal", team, actor%11, m.Ball.Pos)
				// 12FF8 republishes the scoreboard and ages the temporary notice
				// a second time on this successful result update only.
				m.publishOriginalHUD()
			}
			team := (m.originalPeriod.Attempt - 1) & 1
			m.ShootoutTaken[team]++
			m.originalRuleState, m.originalRuleWait = 45, 100
		} else if m.originalRuleWait == 49 {
			m.originalAssignActiveState(54)
		}
	case 45:
		m.updateOriginalPenaltyRound()
	}
}

func (m *Match) originalCompletePeriod() {
	p := &m.originalPeriod
	if p.HalfFlag == 0 || !p.ExtraStage && p.ExtraAllowed && m.Teams[0].Score == m.Teams[1].Score {
		m.Phase = HalfTime
		return
	}
	m.Phase = FullTime
	m.Winner = -1
	if m.Teams[0].Score > m.Teams[1].Score {
		m.Winner = 0
	} else if m.Teams[1].Score > m.Teams[0].Score {
		m.Winner = 1
	}
}

// ContinueOriginalPeriod is the outer A01E/A056/A084 flow after an ordinary
// continue-menu choice. It performs setup only; no A474 world update executes.
func (m *Match) ContinueOriginalPeriod() bool {
	if m.Phase != HalfTime {
		return false
	}
	// 6B4E draws once before the already released menu button enters its
	// press wait. The natural capture stops at6B5C to verify that boundary.
	OriginalRandomWord(&m.originalRandomState)
	p := &m.originalPeriod
	if p.HalfFlag == 0 {
		m.Half++
		p.HalfFlag = 1
		p.Limit = p.FullLimit
	} else {
		if p.ExtraStage || !p.ExtraAllowed || m.Teams[0].Score != m.Teams[1].Score {
			return false
		}
		p.ClockOffset = uint32(m.ClockTicks)
		p.ExtraStage = true
		p.FullLimit = p.RegulationFull / 3
		p.Limit = p.FullLimit >> 1
		p.HalfFlag = 0
		m.originalClock = 0
		m.Half = 3
	}
	m.originalPeriodInjuryTicks = 0
	m.originalKickoffTeam = p.InitialKickoff
	if p.HalfFlag != 0 {
		m.originalKickoffTeam = 1 - m.originalKickoffTeam
	}
	m.originalSetupNextHalf()
	// The interval menu consumes a released-control choice. Do not carry
	// a pitch gesture into the new scene's first input sample.
	m.CancelInput()
	return true
}

func (m *Match) originalSetupNextHalf() {
	p := &m.originalPeriod
	p.EndStarted, p.Exit = false, false
	m.Phase, m.originalRuleState, m.originalRuleWait, m.originalClockRunning = Playing, 10, 0, false
	m.originalCenterBall()
	m.originalBallOwner, m.Ball.TrappedBy, m.Ball.LastTeam, m.Ball.LastPlayer = -1, -1, -1, -1
	m.originalPendingSubstitution = OriginalPendingSubstitutionState{Cursor: -1}
	m.originalSubstitutionDisplay = m.originalPendingSubstitution
	m.originalFoulRestart.ControllerLocked = false
	m.originalCornerSelecting = false
	m.originalCornerContinuation = originalCornerContinuation{}
	m.originalResetOfficialsForEntrance()
	m.originalOfficialGestureStarted = false
	m.originalCameraTarget = -2
	m.originalCameraFixedX, m.originalCameraFixedY = 160<<16, 696<<16
	m.originalCameraX, m.originalCameraY = -192<<16, -80<<16
	// Prepare the normal entrance view through fixed native easing. Recorded
	// source callback counts never select or repair this presentation state.
	for step := 0; step < 4; step++ {
		m.updateOriginalCameraTo(456<<16, 696<<16)
	}
	m.updateOriginalCameraTo(m.originalCameraFixedX, m.originalCameraFixedY)
	m.originalDisplayCameraX, m.originalDisplayCameraY = m.originalCameraX, m.originalCameraY
	m.originalGoalFrame.EndsSwapped = m.AttackDirection(0) > 0
	// 14766 clears retained player markers; FBC6 resets the scanner before
	// the new entrance. Preserve unused Y coordinates as the source does.
	m.initializeOriginalScanner()
	for team := range m.Teams {
		m.Teams[team].Formation = m.Teams[team].queuedFormation
		m.Teams[team].selectOwnPlayer(-1)
		if m.Config.Human[team] {
			m.Teams[team].selectOwnPlayer(1)
		}
		// 14766 invokes 1471C for all thirteen reusable records before 14832
		// installs the eleven field actors. Discipline and injury words survive.
		for record := 0; record < 13; record++ {
			actor := &m.Teams[team].Players[0]
			if record < 11 {
				actor = &m.Teams[team].Players[record]
			} else {
				actor = &m.Teams[team].Bench[record-11]
			}
			actor.Facing, actor.AnimationPhase = OriginalDirection(0), 0
			actor.originalState, actor.originalSubstate, actor.originalDirectionHistory = 0, 0, -1
			actor.originalOpaqueWords[0], actor.originalOpaqueWords[1], actor.originalOpaqueWords[2] = 0, 0, 0
			actor.originalGoalCount, actor.originalVisible = 0, false
		}
		for index := range m.Teams[team].Players {
			actor := &m.Teams[team].Players[index]
			if actor.originalHasTargets {
				for target := range actor.originalTargets {
					actor.originalTargets[target].Y = 1392 - actor.originalTargets[target].Y
					if !actor.Keeper && target < 16 {
						actor.originalTargets[target].X = 912 - actor.originalTargets[target].X
					}
				}
			}
			x, y := 80+16*index, 680
			if m.AttackDirection(team) < 0 {
				y = 712
			}
			actor.Pos.X, actor.Pos.Y = originalCoordinateWord(actor.Pos.X, x-250), originalCoordinateWord(actor.Pos.Y, y)
			actor.originalEntryTarget = Vec2{float64(x), float64(y)}
			actor.originalState, actor.originalDirectionHistory, actor.originalStep = 33, -1, originalPace(actor)
			actor.originalOpaqueWords[2] = 1
			actor.SentOff = false
			actor.originalVisible = false
			actor.Vel = Vec2{}
		}
		if fixed := m.initialBoundPrimaryPlayer(team); fixed >= 0 {
			m.Teams[team].selectOwnPlayer(fixed)
		} else if m.Config.HasSquadBindings[team] && m.Config.Human[team] {
			// 14858 seeds the second record before the on-field B4 scan.
			m.Teams[team].selectOwnPlayer(1)
		}
	}
	// 11E7C literally visits22 contiguous records: thirteen A records and
	// B0..8. Keep that source range instead of silently fixing its bench gap.
	for record := 0; record < 22; record++ {
		team, index := 0, record
		if record >= 13 {
			team, index = 1, record-13
		}
		actor := &m.Teams[team].Players[0]
		if index < 11 {
			actor = &m.Teams[team].Players[index]
		} else {
			actor = &m.Teams[team].Bench[index-11]
		}
		if actor.originalInjuryWord != 0 || actor.originalBookings == 2 {
			m.originalRemoveDepartingPlayer(actor)
			actor.SentOff = actor.originalBookings == 2
		}
	}
	m.Message, m.MessageTicks = "", 0
	// F71A clears every HUD caption during field setup, before the first
	// resumed world update can publish another notice.
	m.originalPublishedMessage, m.originalPersistentMessage = "", ""
	m.originalHUDPublished = true
	m.emit("match_start", -1, -1, m.Ball.Pos)
}

// originalResetOfficialsForEntrance follows EB62 for both reusable officials.
// Integer coordinate/pace writes retain fractions and leave movement words intact.
func (m *Match) originalResetOfficialsForEntrance() {
	if m.Config.Practice || !m.originalHasOfficial {
		return
	}
	for side, p := range []*Player{&m.originalOfficial, &m.originalLinesman} {
		if side == 1 && !m.originalHasLinesman {
			continue
		}
		x := 384
		if side == 1 {
			x = 928
		}
		p.Pos.X, p.Pos.Y = originalCoordinateWord(p.Pos.X, x), originalCoordinateWord(p.Pos.Y, 696)
		p.Facing, p.AnimationPhase = OriginalDirection(0), 0
		p.originalState, p.originalDirectionHistory = 0, -1
		p.originalPaceFixed = p.originalPaceFixed&65535 | 140<<16
		p.originalVisible = false
	}
	m.originalOfficialReady = false
}

func (m *Match) originalStartPenaltyContest() {
	p := &m.originalPeriod
	p.PenaltyContest = true
	p.Attempt = 0
	p.ShooterNumbers = [2]int{10, 10}
	p.ContestInitialScores = [2]int{m.Teams[0].Score, m.Teams[1].Score}
	m.ShootoutScore, m.ShootoutTaken = [2]int{}, [2]int{}
	m.originalRuleState, m.originalRuleWait = 45, 200
}

func (m *Match) originalContestShooter(team, number int) (int, int, bool) {
	for attempt := 0; attempt < 12; attempt++ {
		for index := range m.Teams[team].Players {
			actor := &m.Teams[team].Players[index]
			if actor.Number-1 == number && actor.originalState != 0 && actor.originalInjuryWord == 0 {
				next := number - 1
				if next <= 0 {
					next = 12
				}
				return index, next, true
			}
		}
		number--
		if number <= 0 {
			number = 12
		}
	}
	return 0, number, false
}

func (m *Match) updateOriginalPenaltyRound() {
	m.originalRuleWait--
	if m.originalRuleWait > 0 {
		if m.originalRuleWait == 99 {
			m.originalAssignActiveState(54)
		}
		return
	}
	p := &m.originalPeriod
	remaining := (11 - p.Attempt) >> 1
	difference := m.Teams[0].Score - m.Teams[1].Score
	if difference < 0 {
		difference = -difference
	}
	ended := remaining > 0 && difference > remaining || remaining <= 0 && p.Attempt&1 == 0 && difference >= 1
	if ended {
		p.PenaltyContest = false
		return
	}
	p.Attempt++
	team := (p.Attempt - 1) & 1
	index, next, found := m.originalContestShooter(team, p.ShooterNumbers[team])
	if !found {
		return
	}
	p.ShooterNumbers[team] = next
	m.originalFoul.Team, m.originalFoul.Victim = team, team*11+index
	actor := &m.Teams[team].Players[index]
	actor.originalState, actor.originalDirectionHistory = 28, -1
	m.Teams[0].Players[0].originalSubstate, m.Teams[1].Players[0].originalSubstate = 0, 0
	m.originalRuleState = 33
	// Public diagnostics mirror the actual source rotation; source scores
	// themselves include successful penalty kicks, as12FD8/12FDE write them.
	m.shootoutTeam = team
	m.Restart = Restart{Kind: Penalty, Team: team, Player: index, Pos: Vec2{456, 136}}
}

func (m *Match) OriginalPenaltyContestActive() bool { return m.originalPeriod.PenaltyContest }

func (m *Match) originalPeriodRemainingSeconds() int {
	if m.originalClock >= m.originalPeriod.Limit {
		return 0
	}
	return int(m.originalPeriod.Limit-m.originalClock) / TPS
}
