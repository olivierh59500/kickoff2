package engine

import (
	"fmt"
	"math"
)

// New starts a deterministic match with the supplied options.
func New(config Config) *Match {
	return newMatch(config, nil)
}

// NewWithOriginalRandomState continues the shared source menu/tournament
// generator before wind and referee selection, without native seed warmups.
func NewWithOriginalRandomState(config Config, state [3]uint16) *Match {
	return newMatch(config, &state)
}

func newMatch(config Config, initialRandom *[3]uint16) *Match {
	prepared := prepareMatch(config, initialRandom)
	return prepared.Start(config.StartDownfield, 0)
}

func prepareMatch(config Config, initialRandom *[3]uint16) *MatchPreparation {
	if config.Seed == 0 {
		config.Seed = 1990
	}
	if config.HalfSeconds <= 0 {
		config.HalfSeconds = DefaultConfig().HalfSeconds
	}
	if config.Speed <= 0 {
		config.Speed = 1
	}
	config.Speed = clamp(config.Speed, 0.25, 1.5)
	config.Wind = clampInt(config.Wind, 0, 3)
	config.Pitch = Pitch(clampInt(int(config.Pitch), 0, 4))
	config.LeagueSkill = clampInt(config.LeagueSkill, 0, 2)
	m := &Match{Config: config, rng: config.Seed, Half: 1, Winner: -1, aftertouchTeam: -1, originalBallOwner: -1, originalBallDrawEnabled: true}
	m.originalVideo.nameTeam, m.originalVideo.nameActor = -1, -1
	m.initializeOriginalScanner()
	m.originalHasLinesman, m.originalLinesmanFlag = true, -1
	m.originalLinesman = Player{Pos: Vec2{928, 696}, Facing: OriginalDirection(0), originalPaceFixed: 140 << 16, originalDirectionHistory: -1}
	m.originalPendingSubstitution.Cursor = -1
	m.originalSubstitutionDisplay.Cursor = -1
	m.originalSubstitution.Player, m.originalSubstitution.Reserve = -1, -1
	m.originalPeriod = OriginalPeriodState{Limit: uint32(config.HalfSeconds * TPS), FullLimit: uint32(2 * config.HalfSeconds * TPS), RegulationFull: uint32(2 * config.HalfSeconds * TPS), ExtraAllowed: config.ExtraTime}
	m.originalLastShooter, m.originalCameraTarget = -1, -1
	m.originalThrower = -1
	m.originalOfficialReady = config.Practice
	m.originalOfficialsEnabled, m.originalHasOfficial = true, true
	// FB1E->EB2E initializes the enabled referee at384,696, pace140,
	// action0, heading0, phase0/history-1 and ready0 before any live update.
	m.originalOfficial = Player{Pos: Vec2{384, 696}, Facing: OriginalDirection(0), originalPaceFixed: 140 << 16, originalState: 0, originalDirectionHistory: -1}
	m.originalDirectionInputHistory = initialOriginalInputHistory(config)
	// Source 0x1694a places the supplied longword in the first two words.
	m.originalRandomState = [3]uint16{uint16(config.Seed >> 16), uint16(config.Seed), 0}
	m.originalClockRunning = true
	switch {
	case config.HalfSeconds <= 180:
		m.originalFatigueMask = 63
	case config.HalfSeconds <= 300:
		m.originalFatigueMask = 255
	case config.HalfSeconds <= 600:
		m.originalFatigueMask = 511
	default:
		m.originalFatigueMask = 1023
	}
	if initialRandom != nil {
		m.originalRandomState = *initialRandom
	} else {
		for i := 0; i < 20; i++ {
			OriginalRandomWord(&m.originalRandomState)
		}
	}
	m.Config.Pitch = OriginalPitchChoice(config.Pitch, &m.originalRandomState)
	m.originalWindX, m.originalWindY = OriginalWind(config.Wind, &m.originalRandomState)
	m.originalPALWait = OriginalSpeedWait(config.Speed)
	// 763A chooses a standard tactic for each side without an input pointer,
	// including the parked practice opponent, before 6D52 selects a referee.
	var assigned [2]OriginalTacticState
	for team := range assigned {
		assigned[team] = materializeOriginalTacticBank(&m.Config, team)
	}
	initialTactics := OriginalInitialTactics(config.Human, assigned, &m.originalRandomState)
	referee := config.Referee
	if referee < 0 {
		referee = int(OriginalRandomWord(&m.originalRandomState) % uint16(len(Referees)))
	}
	referee = clampInt(referee, 0, len(Referees)-1)
	m.Referee = Referees[referee]
	return &MatchPreparation{match: m, tactics: initialTactics}
}

func (p *MatchPreparation) Start(down bool, kickoff int) *Match {
	if p.started {
		return p.match
	}
	p.started = true
	m := p.match
	config := m.Config
	config.StartDownfield = down
	m.Config.StartDownfield = down
	if config.Practice {
		kickoff = 0
	}
	kickoff = clampInt(kickoff, 0, 1)
	m.originalPeriod.InitialKickoff = kickoff
	initialTactics := p.tactics
	for t := range m.Teams {
		m.Config.FixedPlayer[t] = clampInt(config.FixedPlayer[t], 0, 10)
		if !m.Config.Human[t] {
			m.Config.FixedPlayer[t], m.Config.Cooperative[t] = 0, false
		}
		if m.Config.Cooperative[t] && !config.HasSquadBindings[t] {
			m.Config.PartnerPlayer[t] = clampInt(config.PartnerPlayer[t], 1, 10)
			if m.Config.FixedPlayer[t] == m.Config.PartnerPlayer[t] {
				m.Config.FixedPlayer[t] = 0
			}
		}
		if config.HasSquadBindings[t] {
			m.Config.FixedPlayer[t], m.Config.PartnerPlayer[t] = 0, 0
			if m.Config.Human[t] {
				for slot := 1; slot < 11; slot++ {
					switch config.SquadBindings[t][slot] {
					case 1:
						m.Config.FixedPlayer[t] = slot
					case 2:
						m.Config.PartnerPlayer[t] = slot
					}
				}
			}
		}
		team := &m.Teams[t]
		team.Name = originalRuntimeName(config.TeamNames[t])
		if team.Name == "" {
			team.Name = []string{"TEAM A", "TEAM B"}[t]
		}
		team.originalTacticSlots, team.originalTacticSlot, team.originalTacticCount = initialTactics[t].Slots, initialTactics[t].Active, initialTactics[t].Count
		id := initialTactics[t].Slots[initialTactics[t].Active]
		team.Formation = Formation(clampInt(int(config.Formation[t]), 0, int(FormationCount)-1))
		if id < OriginalCustomTacticID {
			team.Formation = originalFormationFromIndex(id)
		}
		m.Config.Formation[t] = team.Formation
		team.queuedFormation = team.Formation
		team.selectOwnPlayer(-1)
		if m.Config.Human[t] {
			team.selectOwnPlayer(10)
		}
		for i := range team.Players {
			base := 0.45 + float64(clampInt(config.Skill[t], 0, 4))*0.09
			// The same roster shape is used for both sides at the same skill level.
			variation := float64((i*17+11)%23)/100 - 0.11
			a := Attributes{Pace: clamp(base+variation, 0, 1), Stamina: clamp(base-variation/2, 0, 1),
				Aggression: clamp(0.5+variation, 0, 1), Resilience: base, Passing: base,
				Shooting: clamp(base+variation/2, 0, 1), Tackling: clamp(base-variation, 0, 1)}
			team.Players[i] = Player{Name: fmt.Sprintf("PLAYER %02d", i+1), Number: i + 1, Keeper: i == 0,
				Stamina: 1, Attributes: a, Facing: Vec2{0, float64(m.AttackDirection(t))}, lastTouch: -1000}
		}
		m.loadOriginalRoster(t)
		pacePercent := OriginalInitialPacePercent(config, t)
		m.Config.PacePercent[t] = pacePercent
		for i := range team.Players {
			p := &team.Players[i]
			p.originalState = 1
			p.originalOpaqueWords[2] = 1
			p.originalPaceFixed = uint32(originalPace(p)*pacePercent/100) << 16
			p.originalStaminaWord = int(math.Round(p.Attributes.Stamina * 100))
			p.originalAggressionThreshold = originalInitialAggressionThreshold(p)
			p.originalDirectionHistory = -1
		}
		for i := range team.Bench {
			p := &team.Bench[i]
			p.originalPaceFixed = uint32(originalPace(p)*pacePercent/100) << 16
			p.originalStaminaWord = int(math.Round(p.Attributes.Stamina * 100))
			p.originalAggressionThreshold = originalInitialAggressionThreshold(p)
			p.originalDirectionHistory = -1
		}
		m.reloadOriginalTactic(t)
	}
	m.resetPositions()
	m.beginRestart(KickOff, kickoff, Vec2{PitchWidth / 2, PitchHeight / 2}, TPS)
	if config.Practice {
		m.Phase = Playing
		m.Ball = Ball{Pos: Vec2{PitchWidth / 2, PitchHeight / 2}, LastTeam: -1, LastPlayer: -1, TrappedBy: -1}
		// 0x126f8/0x12720 starts the closest kickoff actor in state31.
		// Its automatic source pass selects the eventual practice player;
		// there is no special placement of shirt11 beside the ball.
		for index := range m.Teams[0].Players {
			p := &m.Teams[0].Players[index]
			if target, ok := m.restartTacticPosition(0, index, KickOff, true, m.Ball.Pos, m.AttackDirection(0)); ok {
				p.Pos = target
			}
			p.Facing = OriginalDirection(OriginalDirectionIndex(Vec2{math.Floor(m.Ball.Pos.X) - math.Floor(p.Pos.X), math.Floor(m.Ball.Pos.Y) - math.Floor(p.Pos.Y)}))
		}
		if taker := m.originalNearest(0, -1); taker >= 1 {
			m.Teams[0].Players[taker].originalState = 31
			m.Restart.Player = taker
			if m.Config.Human[0] {
				m.Teams[0].selectOwnPlayer(taker)
			}
		}
		// The original parks inactive practice opponents beside the touchline.
		for i := 1; i < 11; i++ {
			m.Teams[1].Players[i].Pos = Vec2{-170 + float64(i)*16, 680}
			m.Teams[1].Players[i].originalState = 0
		}
		m.Message = "PRACTICE"
		m.Teams[1].selectOwnPlayer(-1)
	} else {
		m.beginOriginalOpening()
	}
	for team := range m.Teams {
		if fixed := m.initialBoundPrimaryPlayer(team); fixed >= 0 && !(m.Config.Practice && team == 1) {
			m.Teams[team].selectOwnPlayer(fixed)
		} else if m.Config.HasSquadBindings[team] && m.Config.Human[team] && !m.Config.Practice {
			// 1484E starts at shirt one, then lets each later matching B4
			// overwrite that pointer while it walks the eleven records.
			m.Teams[team].selectOwnPlayer(1)
		}
	}
	m.originalCameraX = int32(math.Round((m.Ball.Pos.X - 176) * originalFixedScale))
	m.originalCameraY = int32(math.Round((m.Ball.Pos.Y - 128) * originalFixedScale))
	if !config.Practice {
		// 11072/1107A reset the source ground scroll before its video
		// callback follows the entrance rows. Field offsets subtract192/80.
		m.originalCameraX, m.originalCameraY = -192<<16, -80<<16
	}
	m.originalDisplayCameraX, m.originalDisplayCameraY = m.originalCameraX, m.originalCameraY
	for team := range m.Teams {
		for index := range m.Teams[team].Players {
			p := &m.Teams[team].Players[index]
			p.originalVisible = m.originalPlayerVisible(p)
			if config.Practice && p.originalState != 0 {
				// The source has already cached actor distances during its
				// entrance before state31 starts; fresh practice skips that walk.
				p.originalBallDistance = originalDistanceSquared(p.Pos, m.Ball.Pos)
				p.originalDistanceReady = true
			}
		}
	}
	return m
}

// AttackDirection returns -1 for an attack towards the upper goal or +1 for lower.
func (m *Match) AttackDirection(team int) int {
	d := -1
	if team == 1 {
		d = 1
	}
	if m.Half%2 == 0 {
		d = -d
	}
	if m.Config.StartDownfield {
		d = -d
	}
	return d
}

// DefendingTeam returns the team defending the selected end line.
func (m *Match) DefendingTeam(top bool) int {
	for t := range m.Teams {
		if (m.AttackDirection(t) == 1) == top {
			return t
		}
	}
	return 0
}

// ClockSeconds is the elapsed simulated match time, excluding dead-ball pauses.
func (m *Match) ClockSeconds() int { return m.ClockTicks / TPS }

// RemainingSeconds retains F74A/F884's subtraction before unsigned division.
// Subtracting rounded elapsed seconds would display one extra second between
// whole-second boundaries. ClockTicks is cumulative across native periods.
func (m *Match) RemainingSeconds() int {
	return m.originalPeriodRemainingSeconds()
}

// Update advances a source game-loop iteration, or polls a blocking selector.
// Video-rate hosts use UpdatePALControllers to honor the original speed waits.
func (m *Match) Update(input [2]Input) {
	m.update(input, false)
}

func (m *Match) update(input [2]Input, modalHistorySampled bool) {
	defer func() {
		m.originalHasQueuedSelectorInputs = false
		m.originalQueuedModalPolls = m.originalQueuedModalPolls[:0]
		m.originalSubstitutePolls = m.originalSubstitutePolls[:0]
	}()
	if !m.cooperativeFrame {
		m.cooperativeInputs = [2]Input{}
	}
	m.Events = m.Events[:0]
	m.originalUpdateCompleted = false
	if m.Phase == HalfTime {
		primary := 0
		if !m.Config.Human[0] && m.Config.Human[1] {
			primary = 1
		}
		if input[primary].Fire && !m.previous[primary].Fire || m.partnerContinuePressed() {
			m.ContinueOriginalPeriod()
		}
		m.previous, m.cooperativePrevious = input, m.cooperativeInputs
		return
	}
	if m.originalSubstitutionBlocked {
		m.pollOriginalBlockedInput(input, modalHistorySampled)
		if m.originalHasSubstituteChoice {
			m.originalChooseInjuryReserve(m.originalSubstituteChoice)
			m.originalSubstitutionKeyboardPoll()
			if !m.originalPALHistorySampled {
				m.updateOriginalInputHistory()
			}
			m.previous, m.cooperativePrevious = m.controls, m.cooperativeInputs
			m.originalUpdateCompleted = true
		}
		m.updateOriginalDisplayCamera()
		m.updateOriginalCamera()
		return
	}
	if m.originalOpeningSoundPending {
		m.emit("match_start", -1, -1, m.Ball.Pos)
		m.originalOpeningSoundPending = false
	}
	if m.Phase == FullTime {
		return
	}
	if m.originalCornerSelecting {
		m.pollOriginalCorner(input, modalHistorySampled)
		return
	}
	m.Tick++
	m.originalUpdateCompleted = true
	retargeted := m.originalCameraRetargeted
	if retargeted {
		m.originalCameraX, m.originalCameraY = m.originalDisplayCameraX, m.originalDisplayCameraY
		m.originalActorCameraAtEntry = true
		m.originalCameraRetargeted = false
	}
	defer m.finishOriginalDisplayCamera()
	// Entrance rows run before the next main-wait video callback. Cache its
	// entry target: a manager retarget takes effect at the following boundary.
	cameraAfterActors := !retargeted && (m.originalRuleState == 10 || m.originalRuleState == 11 || !m.originalClockRunning && (m.originalRuleState == 1 || m.originalRuleState == 2))
	cameraTarget := m.originalCameraTarget
	cameraX, cameraY := m.originalCameraFocus()
	if !cameraAfterActors {
		if !m.originalActorCameraAtEntry {
			m.updateOriginalCameraTo(cameraX, cameraY)
		}
	}
	m.originalActorCameraAtEntry = false
	if m.Phase != Playing {
		// Legacy native restart/replay phases do not enter12560's HUD tail.
		// Keep their existing announcement expiry through the same publisher.
		m.publishOriginalHUD()
	}
	m.prepareOriginalControls(input)
	input = m.controls
	m.beginOriginalVideo(cameraX, cameraY)
	defer func() {
		if !m.originalCornerSelecting {
			m.finishOriginalVideo()
		}
	}()
	switch m.Phase {
	case Kickoff, SetPiece:
		if m.Phase == Kickoff {
			m.updateOriginalOfficial()
		}
		m.updateControllerRestart(input)
	case Goal:
		m.phaseTicks--
		m.updateBall()
		m.recordOriginalBallVideo()
		if m.phaseTicks <= 0 {
			m.resetPositions()
			m.beginRestart(KickOff, 1-m.Restart.Team, Vec2{PitchWidth / 2, PitchHeight / 2}, TPS)
		}
	case Playing:
		// The source main loop integrates the ball at 0xfcce before invoking
		// player logic at 0xfce2. New touches take effect on the following step.
		m.updateBall()
		m.recordOriginalBallVideo()
		if view := m.originalPendingSubstitution; view.Viewing && view.Cursor >= 0 && view.Cursor < 22 {
			m.originalCameraTarget = view.Cursor
		}
		m.updateOriginalOfficial()
		m.recordOriginalOfficialVideo()
		m.updateOriginalLinesman()
		if !m.completeOriginalPlayingUpdate(cameraAfterActors, cameraX, cameraY, cameraTarget) {
			return
		}
	}
	m.finishOriginalUpdate()
}

func (m *Match) emit(kind string, team, player int, pos Vec2) {
	m.Events = append(m.Events, Event{Tick: m.Tick, Kind: kind, Team: team, Player: player, Pos: pos,
		SoundPractice: m.Config.Practice, SoundCrowdMuted: m.Config.CrowdMuted, HasSoundPreferences: true})
}

func (m *Match) notice(message string, ticks int) { m.Message = message; m.MessageTicks = ticks }

// PublishedMessage is the announcement currently shown on the field. Match
// rules can queue another notice after this frame's HUD publication.
func (m *Match) PublishedMessage() string {
	if !m.originalHUDPublished {
		if m.Message != "" {
			return m.Message
		}
		return m.originalPersistentMessage
	}
	if m.originalPublishedMessage == "" {
		return m.originalPersistentMessage
	}
	return m.originalPublishedMessage
}

// publishOriginalHUD follows F82E, normally after actors/clock at1256A.
// A successful penalty-contest result also calls it at12FF8 after scoring.
func (m *Match) publishOriginalHUD() {
	if m.MessageTicks > 0 {
		m.MessageTicks--
		if m.MessageTicks == 0 {
			m.Message = ""
		}
	}
	m.originalPublishedMessage, m.originalHUDPublished = m.Message, true
}

func (m *Match) random() float64 {
	return float64(OriginalRandomWord(&m.originalRandomState)) / 65536
}

func (m *Match) resetPositions() {
	for t := range m.Teams {
		m.Teams[t].Formation = m.Teams[t].queuedFormation
		for i := range m.Teams[t].Players {
			p := &m.Teams[t].Players[i]
			p.Pos = m.homePosition(t, i)
			p.Vel = Vec2{}
			p.Facing = Vec2{0, float64(m.AttackDirection(t))}
			p.ActionTicks = 0
			p.jumpTicks = 0
			p.tackleTicks = 0
			p.shotPauseTicks = 0
			p.JumpHeight = 0
			if p.Keeper {
				y := 18.0
				if m.AttackDirection(t) < 0 {
					y = 1374
				}
				for target := range p.originalTargets {
					p.originalTargets[target] = Vec2{456, y}
				}
				p.originalHasTargets = true
			}
		}
	}
	// The opening forwards are positioned around the spot without encroachment.
	t := 0
	if m.Half%2 == 0 {
		t = 1
	}
	m.Teams[t].Players[10].Pos = Vec2{PitchWidth / 2, PitchHeight/2 - float64(m.AttackDirection(t))*16}
}

func (m *Match) checkClock() {
	m.originalCheckPeriodClock()
}

func (m *Match) finish() {
	m.Phase = FullTime
	m.clearBallControl()
	if m.Teams[0].Score > m.Teams[1].Score {
		m.Winner = 0
	} else if m.Teams[1].Score > m.Teams[0].Score {
		m.Winner = 1
	}
	m.notice("FULL TIME", 0)
	m.emit("whistle", -1, -1, m.Ball.Pos)
}

func (m *Match) clearBallControl() {
	m.originalFoulRestart.ControllerLocked = false
	m.originalHasQueuedSelectorInputs = false
	m.originalPracticeGoalPending = false
	m.originalCornerSelecting = false
	m.originalCornerContinuation = originalCornerContinuation{}
	m.originalCornerCountdown, m.originalCornerHeld, m.originalCornerTaker = 0, 0, -1
	m.originalBallDrawEnabled = true
	m.originalGoalFrame = OriginalGoalFrameState{EndsSwapped: m.AttackDirection(0) > 0}
	m.originalRuleState, m.originalRuleWait, m.originalThrowCountdown = 0, 0, 0
	m.Ball.TrappedBy = -1
	m.originalBallOwner = -1
	m.ballCurve = 0
	m.aftertouchTicks = 0
	m.aftertouchLateral, m.aftertouchVertical = 0, 0
	m.aftertouchNewShot = false
	m.keeperHold = 0
	m.pendingTrapInput = [2]bool{}
}

// CancelInput discards action transitions and an unfinished restart charge after
// a pause, focus change, or screen transition. A human's trapped ball waits for a
// fresh direction or button action, rather than interpreting cancellation as a
// pass. Match phase, clock, ball physics and player positions are preserved.
func (m *Match) CancelInput() {
	m.previous = [2]Input{}
	m.controls = [2]Input{}
	m.cooperativeInputs = [2]Input{}
	m.cooperativePrevious = [2]Input{}
	for controller, word := range initialOriginalInputHistory(m.Config) {
		if controller < 2 || controller == 2 && m.Config.Practice {
			m.originalDirectionInputHistory[controller] = word
		}
	}
	m.Restart.Armed = false
	m.Restart.ChargeTicks = 0
	// Host focus/lifecycle cancellation also covers the source match restart
	// actions, whose charge lives outside the legacy Restart record.
	for team := range m.Teams {
		if !m.Config.Human[team] {
			continue
		}
		for index := range m.Teams[team].Players {
			p := &m.Teams[team].Players[index]
			if p.originalState == 57 && m.originalFoulRestart.Started == 0 {
				p.originalState = 56
				m.originalFoulRestart.Height = 0
			}
			if p.originalState == 42 && m.originalFoulRestart.PenaltyHeld != 0 {
				m.originalFoulRestart.PenaltyHeld = 0
				p.originalStep, p.Vel = 0, Vec2{}
			}
		}
	}
	m.pendingTrapInput = [2]bool{}
	if owner := m.Ball.TrappedBy; owner >= 0 && owner%11 != 0 && m.Config.Human[owner/11] {
		m.pendingTrapInput[owner/11] = true
	}
}

// Substitute requests the first unused reserve and preselects a field record.
// The normal source stoppage/confirmation/departure chain performs the swap.
func (m *Match) Substitute(team, player int) bool {
	if team < 0 || team > 1 {
		return false
	}
	for reserve := range m.Teams[team].BenchUsed {
		if !m.Teams[team].BenchUsed[reserve] {
			return m.SubstituteReserve(team, player, reserve)
		}
	}
	return false
}

// SubstituteReserve requests a reserve and remembers the intended cursor.
// It never changes player identity immediately. The original selector includes
// the keeper; entry eligibility follows its injury and booking words.
func (m *Match) SubstituteReserve(team, player, reserve int) bool {
	if !m.originalEligibleSubstitutionActor(team, player) || reserve < 0 || reserve > 1 {
		return false
	}
	if !m.RequestOriginalSubstitution(team, reserve) {
		return false
	}
	m.originalPendingSubstitution.Cursor = team*11 + player
	return true
}

func rotate(v Vec2, a float64) Vec2 {
	s, c := math.Sincos(a)
	return Vec2{v.X*c - v.Y*s, v.X*s + v.Y*c}
}
