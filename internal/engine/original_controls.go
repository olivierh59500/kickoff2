package engine

import "math"

// OriginalPlayerState contains the source fields needed at a PAL update
// boundary. State uses the original player-action index, not the UI animation.
type OriginalPlayerState struct {
	KeeperTarget                              Vec2
	HasKeeperTarget                           bool
	OpaqueWords                               [4]uint16
	HasOpaqueWords                            bool
	RestartTarget                             Vec2
	HasRestartTarget                          bool
	FoulPoints, Bookings, InjuryWord          uint16
	RecoveryLimit                             uint16
	ShirtIndex                                int
	TackleHit, HasFoulContext                 bool
	EntryTarget                               Vec2
	HasEntryTarget                            bool
	Name                                      string
	HasName                                   bool
	NameBytes                                 [16]byte
	HasNameBytes                              bool
	GoalCount                                 uint16
	CelebrationWait                           int16
	HasGoalContext                            bool
	AggressionThreshold, Tackling, Resilience int
	HasDiscipline                             bool
	Targets                                   [20]Vec2
	HasTargets                                bool
	AnimationPhase                            uint16
	Substate                                  int
	Visible, HasVisibility                    bool
	X, Y                                      int32
	Direction, Step, Pace, State              int
	BallDistance                              int32
	TouchCooldown, OwnerTicks                 int
	Shooting, Passing                         int
	PaceFixed                                 uint32
	Stamina, DirectionHistory                 int
}

// OriginalOfficialState preserves the observed referee record used by the
// goal/kickoff readiness gate. It is independent of the footballer arrays.
type OriginalOfficialState struct {
	GestureStarted               bool
	X, Y                         int32
	Direction, Step, Pace, State int
	DirectionHistory             int
	AnimationPhase               uint16
}

// OriginalRuntimeSnapshot describes one observed original update boundary.
// BallOwner encodes a team possession pointer; LastTeam/LastPlayer preserve the
// separate source last-touch pointer after a shot has cleared possession.
type OriginalRuntimeSnapshot struct {
	CrowdMuted                            bool
	HasSoundPreferences                   bool
	HUDMessage                            string
	HUDTicks                              int
	HasHUD                                bool
	HUDPublishedMessage                   string
	HasHUDPublished                       bool
	HUDPersistentMessage                  string
	HasHUDPersistent                      bool
	Linesman                              OriginalOfficialState
	LinesmanFlag                          int
	HasLinesman                           bool
	Scanner                               OriginalScannerState
	HasScanner                            bool
	Tactics                               [2]OriginalTacticState
	HasTactics                            [2]bool
	AssignedTacticData                    [2][4][]byte
	AssignedTacticNames                   [2][4]string
	HasAssignedTacticData                 [2]bool
	ThrowCountdown                        int
	HasThrowCountdown                     bool
	ThrowInTaker                          int
	HasThrowInTaker                       bool
	Period                                OriginalPeriodState
	HasPeriod                             bool
	SubstitutionDisplay                   OriginalPendingSubstitutionState
	HasSubstitutionDisplay                bool
	PendingSubstitution                   OriginalPendingSubstitutionState
	HasPendingSubstitution                bool
	FoulRestart                           OriginalFoulRestartState
	HasFoulRestart                        bool
	AppliedInjuryTicks, PeriodInjuryTicks uint32
	HasInjuryTime                         bool
	CameraHoldX, CameraHoldY              int32
	HasCameraHold                         bool
	Bench                                 [2][2]OriginalPlayerState
	HasBench                              [2][2]bool
	BenchUsed                             [2][2]bool
	HasBenchUsage                         bool
	Substitution                          OriginalSubstitutionState
	HasSubstitution                       bool
	Foul                                  OriginalFoulState
	HasFoulContext                        bool
	ActorCameraX, ActorCameraY            int32
	HasActorCamera                        bool
	ActorCameraAtEntry                    bool
	CameraRetargeted                      bool
	CameraFixedX, CameraFixedY            int32
	HasFixedCamera                        bool
	Official                              OriginalOfficialState
	HasOfficial                           bool
	LastShooter, KickoffTeam              int
	HasLastShooter, HasKickoffTeam        bool
	CameraTarget                          int
	HasCameraTarget                       bool
	OfficialReady, OfficialsEnabled       bool
	HasOfficialContext                    bool
	KeeperHold, KeeperRepeatedCatch       int
	KeeperDrift                           int32
	KeeperAnimationCycles                 int
	HasKeeperContext                      bool
	WindX, WindY                          int32
	HasWind                               bool
	JoystickHistory                       [4]int16
	HasJoystickHistory                    bool
	RuleState, RuleWait                   int
	HasRuleState                          bool
	Scores                                [2]int
	HasScores                             bool
	Advertising                           OriginalAdvertisingState
	HasAdvertising                        bool
	BallDrawEnabled, HasBallDrawFlag      bool
	GoalFrame                             OriginalGoalFrameState
	HasGoalFrame                          bool
	Ball                                  OriginalBallState
	Players                               [2][11]OriginalPlayerState
	Controlled                            [2]int
	ControlledOpposing                    [2]bool
	ControlHandoffSuppressed              bool
	BallOwner, LastTeam, LastPlayer       int
	RandomState                           [3]uint16
	Clock                                 uint32
	FatigueMask                           uint16
	ClockRunning                          bool
	CameraX, CameraY                      int32
	HasCamera                             bool
}

// RestoreOriginalPracticeSnapshot imports only the initial state of an
// independent practice trace. Match.Update then predicts every later update.
func (m *Match) RestoreOriginalPracticeSnapshot(snapshot OriginalRuntimeSnapshot) {
	m.Config.Practice = true
	m.restoreOriginalRuntimeSnapshot(snapshot)
}

// RestoreOriginalMatchSnapshot imports a live full-match boundary. Direction is
// team zero's attack direction (-1 upwards, +1 downwards); no practice shortcuts
// or inactive-opponent rules are enabled for this importer.
func (m *Match) RestoreOriginalMatchSnapshot(snapshot OriginalRuntimeSnapshot, direction int) {
	m.Config.Practice = false
	m.Half = 1
	m.Config.StartDownfield = direction > 0
	m.restoreOriginalRuntimeSnapshot(snapshot)
	m.Config.StartDownfield = (direction > 0) != (m.Half%2 == 0)
}

func (m *Match) restoreOriginalRuntimeSnapshot(snapshot OriginalRuntimeSnapshot) {
	if snapshot.HasSoundPreferences {
		m.Config.CrowdMuted = snapshot.CrowdMuted
	}
	if snapshot.HasHUD {
		m.Message, m.MessageTicks = snapshot.HUDMessage, snapshot.HUDTicks
	}
	m.originalPublishedMessage, m.originalHUDPublished = snapshot.HUDPublishedMessage, snapshot.HasHUDPublished
	m.originalPersistentMessage = snapshot.HUDPersistentMessage
	m.originalVideo = originalVideoState{nameTeam: snapshot.LastTeam, nameActor: snapshot.LastPlayer}
	m.originalActorCameraAtEntry = snapshot.ActorCameraAtEntry
	m.originalCameraRetargeted = snapshot.CameraRetargeted
	m.originalCameraFocusDeferred = false
	m.originalDeferredCameraTarget = -1
	m.originalHasLinesman = snapshot.HasLinesman
	if snapshot.HasLinesman {
		s := snapshot.Linesman
		m.originalLinesman = Player{Pos: Vec2{float64(s.X) / 65536, float64(s.Y) / 65536}, Facing: OriginalDirection(s.Direction), AnimationPhase: s.AnimationPhase, originalStep: s.Step, originalPaceFixed: uint32(s.Pace) << 16, originalDirectionHistory: s.DirectionHistory, originalState: s.State}
		m.originalLinesmanFlag = snapshot.LinesmanFlag
	}
	m.initializeOriginalScanner()
	if snapshot.HasScanner {
		m.originalVideo.scanner = snapshot.Scanner
	}
	for team, present := range snapshot.HasAssignedTacticData {
		if present {
			for slot := 0; slot < 4; slot++ {
				m.Config.AssignedTacticData[team][slot] = append([]byte(nil), snapshot.AssignedTacticData[team][slot]...)
				m.Config.AssignedTacticNames[team][slot] = snapshot.AssignedTacticNames[team][slot]
			}
		}
	}
	for team, present := range snapshot.HasTactics {
		if present {
			m.Teams[team].originalTacticSlots = snapshot.Tactics[team].Slots
			m.Teams[team].originalTacticSlot = snapshot.Tactics[team].Active
			m.Teams[team].originalTacticCount = snapshot.Tactics[team].Count
			if m.Teams[team].originalTacticCount == 0 {
				m.Teams[team].originalTacticCount = 4
			}
			id := snapshot.Tactics[team].Slots[snapshot.Tactics[team].Active]
			if id < OriginalCustomTacticID {
				formation := originalFormationFromIndex(id)
				m.Teams[team].Formation, m.Teams[team].queuedFormation = formation, formation
			}
		}
	}
	if snapshot.HasPeriod {
		m.originalPeriod = snapshot.Period
		m.Half = 1 + snapshot.Period.HalfFlag
		if snapshot.Period.ExtraStage {
			m.Half += 2
		}
		m.Config.ExtraTime = snapshot.Period.ExtraAllowed
	}
	m.originalKeyboardEvents = nil
	m.originalPendingSubstitution = OriginalPendingSubstitutionState{Cursor: -1}
	if snapshot.HasPendingSubstitution {
		m.originalPendingSubstitution = snapshot.PendingSubstitution
	}
	m.originalSubstitutionDisplay = m.originalPendingSubstitution
	if snapshot.HasSubstitutionDisplay {
		m.originalSubstitutionDisplay = snapshot.SubstitutionDisplay
	}
	if snapshot.HasFoulRestart {
		m.originalFoulRestart = snapshot.FoulRestart
	}
	if snapshot.HasInjuryTime {
		m.originalAppliedInjuryTicks, m.originalPeriodInjuryTicks = snapshot.AppliedInjuryTicks, snapshot.PeriodInjuryTicks
	}
	if snapshot.HasFoulContext {
		m.originalFoul = snapshot.Foul
	}
	m.originalSubstitutionBlocked, m.originalHasSubstituteChoice = false, false
	m.originalHasSubstituteInputs, m.originalUpdateCompleted = false, false
	if snapshot.HasCameraHold {
		m.originalCameraHoldX, m.originalCameraHoldY = snapshot.CameraHoldX, snapshot.CameraHoldY
	}
	if snapshot.HasSubstitution {
		m.originalSubstitution = snapshot.Substitution
	}
	for team := range snapshot.Bench {
		if snapshot.HasBenchUsage {
			m.Teams[team].BenchUsed = snapshot.BenchUsed[team]
		}
		for slot, state := range snapshot.Bench[team] {
			if snapshot.HasBench[team][slot] {
				restoreOriginalPlayerState(&m.Teams[team].Bench[slot], state)
			}
		}
	}
	m.originalOpeningSoundPending = false
	m.originalLastShooter, m.originalCameraTarget = -1, -1
	m.originalCameraFixedX, m.originalCameraFixedY = 160<<16, 696<<16
	m.originalKickoffTeam = 0
	m.originalOfficialReady, m.originalOfficialsEnabled = true, false
	m.originalHasOfficial = snapshot.HasOfficial
	if snapshot.HasOfficial {
		s := snapshot.Official
		m.originalOfficial = Player{Pos: Vec2{float64(s.X) / originalFixedScale, float64(s.Y) / originalFixedScale},
			Facing: OriginalDirection(s.Direction), originalStep: s.Step, originalState: s.State,
			originalPaceFixed: uint32(s.Pace) << 16, originalDirectionHistory: s.DirectionHistory, AnimationPhase: s.AnimationPhase}
		m.originalOfficialGestureStarted = s.GestureStarted
	}
	m.originalMatchGoalPending = false
	if snapshot.HasLastShooter {
		m.originalLastShooter = snapshot.LastShooter
	}
	if snapshot.HasKickoffTeam {
		m.originalKickoffTeam = snapshot.KickoffTeam
	}
	if snapshot.HasCameraTarget {
		m.originalCameraTarget = snapshot.CameraTarget
	}
	if snapshot.HasFixedCamera {
		m.originalCameraFixedX, m.originalCameraFixedY = snapshot.CameraFixedX, snapshot.CameraFixedY
	}
	if snapshot.HasOfficialContext {
		m.originalOfficialReady, m.originalOfficialsEnabled = snapshot.OfficialReady, snapshot.OfficialsEnabled
	}
	m.keeperHold, m.originalKeeperGlobalDrift = 0, 0
	m.originalKeeperAnimationCycles, m.originalKeeperRepeatedCatch = 0, 0
	if snapshot.HasKeeperContext {
		m.keeperHold, m.originalKeeperGlobalDrift = snapshot.KeeperHold, snapshot.KeeperDrift
		m.originalKeeperAnimationCycles, m.originalKeeperRepeatedCatch = snapshot.KeeperAnimationCycles, snapshot.KeeperRepeatedCatch
	}
	if snapshot.HasWind {
		m.originalWindX, m.originalWindY = snapshot.WindX, snapshot.WindY
	}
	m.originalPALWait = 0
	m.originalHasQueuedSelectorInputs = false
	m.originalDirectionInputHistory = initialOriginalInputHistory(m.Config)
	if snapshot.HasJoystickHistory {
		m.originalDirectionInputHistory = snapshot.JoystickHistory
	}
	m.originalBallDerivedReady = false
	m.originalPracticeGoalPending = false
	m.originalCornerSelecting = false
	m.originalCornerContinuation = originalCornerContinuation{}
	m.originalCornerCountdown, m.originalCornerHeld, m.originalCornerTaker = 0, 0, -1
	m.originalAdvertising = OriginalAdvertisingState{}
	if snapshot.HasAdvertising {
		m.originalAdvertising = snapshot.Advertising
	}
	m.originalBallDrawEnabled = true
	if snapshot.HasBallDrawFlag {
		m.originalBallDrawEnabled = snapshot.BallDrawEnabled
	}
	m.originalGoalFrame = OriginalGoalFrameState{EndsSwapped: m.AttackDirection(0) > 0}
	if snapshot.HasGoalFrame {
		m.originalGoalFrame = snapshot.GoalFrame
	}
	m.originalRuleState, m.originalRuleWait, m.originalThrowCountdown = 0, 0, 0
	m.originalThrower = -1
	if snapshot.HasThrowInTaker {
		m.originalThrower = snapshot.ThrowInTaker
	}
	if snapshot.HasThrowCountdown {
		m.originalThrowCountdown = snapshot.ThrowCountdown
	}
	if snapshot.HasRuleState {
		m.originalRuleState, m.originalRuleWait = snapshot.RuleState, snapshot.RuleWait
	}
	m.Phase = Playing
	m.Ball.applyOriginalState(snapshot.Ball)
	m.ballCurve = snapshot.Ball.Curve
	m.Ball.TrappedBy = -1
	for team := range m.Teams {
		for index := range m.Teams[team].Players {
			m.RestoreOriginalPlayerState(team, index, snapshot.Players[team][index])
		}
		m.Teams[team].selectOwnPlayer(snapshot.Controlled[team])
		m.Teams[team].ControlledOpposing = snapshot.ControlledOpposing[team]
		if snapshot.HasScores {
			m.Teams[team].Score = snapshot.Scores[team]
		}
	}
	m.originalControlHandoffSuppressed = snapshot.ControlHandoffSuppressed
	m.SetOriginalBallOwner(snapshot.BallOwner)
	if !snapshot.HasThrowInTaker && m.originalRuleState == 3 {
		// Older snapshots omitted the dedicated taker pointer. A unique
		// active throw action still identifies it from this initial state.
		candidate := -1
		for team := range m.Teams {
			for actor := 1; actor < 11; actor++ {
				state := m.Teams[team].Players[actor].originalState
				if state == 9 || state == 10 || state == 29 {
					if candidate != -1 {
						candidate = -2
						break
					}
					candidate = team*11 + actor
				}
			}
			if candidate == -2 {
				break
			}
		}
		if candidate >= 0 {
			m.originalThrower = candidate
		}
	}
	m.Ball.LastTeam, m.Ball.LastPlayer = snapshot.LastTeam, snapshot.LastPlayer
	if snapshot.LastTeam >= 0 && snapshot.LastTeam < 2 && snapshot.LastPlayer >= 0 && snapshot.LastPlayer < 11 {
		m.originalVideo.caption, m.originalVideo.hasCaption = m.originalNameCommand(snapshot.LastTeam, snapshot.LastPlayer), true
	}
	m.SetOriginalRandomState(snapshot.RandomState)
	m.originalClock, m.originalFatigueMask, m.originalClockRunning = snapshot.Clock, snapshot.FatigueMask, snapshot.ClockRunning
	m.ClockTicks = int(snapshot.Clock)
	if snapshot.HasPeriod {
		m.ClockTicks += int(snapshot.Period.ClockOffset)
	}
	if snapshot.HasCamera {
		m.originalCameraX, m.originalCameraY = snapshot.CameraX, snapshot.CameraY
	} else {
		m.originalCameraX = snapshot.Ball.X - 176*65536
		m.originalCameraY = snapshot.Ball.Y - 128*65536
	}
	m.originalDisplayCameraX, m.originalDisplayCameraY = m.originalCameraX, m.originalCameraY
	if snapshot.HasActorCamera {
		m.originalCameraX, m.originalCameraY = snapshot.ActorCameraX, snapshot.ActorCameraY
	}
	for team := range m.Teams {
		for index := range m.Teams[team].Players {
			p := &m.Teams[team].Players[index]
			if snapshot.Players[team][index].HasVisibility {
				p.originalVisible = snapshot.Players[team][index].Visible
			} else {
				p.originalVisible = m.originalPlayerVisible(p)
			}
		}
	}
	m.previous, m.controls = [2]Input{}, [2]Input{}
	if snapshot.HasPeriod && snapshot.Period.Exit {
		m.originalCompletePeriod()
	}
	m.originalFireConsumed = [4]bool{}
	m.aftertouchTicks, m.aftertouchLateral, m.aftertouchVertical = 0, 0, 0
}

// OriginalSnapshot returns source-unit state suitable for an audited trace.
func (m *Match) OriginalSnapshot() OriginalRuntimeSnapshot {
	snapshot := OriginalRuntimeSnapshot{Ball: originalBallState(m.Ball, m.ballCurve),
		Controlled: [2]int{m.Teams[0].Controlled, m.Teams[1].Controlled}, BallOwner: m.originalBallOwner,
		ControlledOpposing:       [2]bool{m.Teams[0].ControlledOpposing, m.Teams[1].ControlledOpposing},
		ControlHandoffSuppressed: m.originalControlHandoffSuppressed,
		LastTeam:                 m.Ball.LastTeam, LastPlayer: m.Ball.LastPlayer, RandomState: m.originalRandomState}
	snapshot.Scanner, snapshot.HasScanner = m.originalVideo.scanner, true
	snapshot.CrowdMuted, snapshot.HasSoundPreferences = m.Config.CrowdMuted, true
	snapshot.HUDMessage, snapshot.HUDTicks, snapshot.HasHUD = m.Message, m.MessageTicks, true
	snapshot.HUDPublishedMessage, snapshot.HasHUDPublished = m.originalPublishedMessage, m.originalHUDPublished
	snapshot.HUDPersistentMessage, snapshot.HasHUDPersistent = m.originalPersistentMessage, true
	l := &m.originalLinesman
	snapshot.Linesman = OriginalOfficialState{X: int32(math.Round(l.Pos.X * 65536)), Y: int32(math.Round(l.Pos.Y * 65536)), Direction: OriginalDirectionIndex(l.Facing), Step: l.originalStep, Pace: originalPace(l), State: l.originalState, AnimationPhase: l.AnimationPhase, DirectionHistory: l.originalDirectionHistory}
	snapshot.HasLinesman, snapshot.LinesmanFlag = m.originalHasLinesman, m.originalLinesmanFlag
	snapshot.LastShooter, snapshot.HasLastShooter = m.originalLastShooter, true
	snapshot.Foul, snapshot.HasFoulContext = m.originalFoul, true
	snapshot.FoulRestart, snapshot.HasFoulRestart = m.originalFoulRestart, true
	snapshot.AppliedInjuryTicks, snapshot.PeriodInjuryTicks, snapshot.HasInjuryTime = m.originalAppliedInjuryTicks, m.originalPeriodInjuryTicks, true
	snapshot.CameraHoldX, snapshot.CameraHoldY, snapshot.HasCameraHold = m.originalCameraHoldX, m.originalCameraHoldY, true
	snapshot.Substitution, snapshot.HasSubstitution = m.originalSubstitution, true
	snapshot.PendingSubstitution, snapshot.HasPendingSubstitution = m.originalPendingSubstitution, true
	snapshot.Period, snapshot.HasPeriod = m.originalPeriod, true
	snapshot.SubstitutionDisplay, snapshot.HasSubstitutionDisplay = m.originalSubstitutionDisplay, true
	snapshot.HasBenchUsage = true
	for team := range m.Teams {
		snapshot.BenchUsed[team] = m.Teams[team].BenchUsed
		for slot, p := range m.Teams[team].Bench {
			snapshot.HasBench[team][slot] = true
			snapshot.Bench[team][slot] = OriginalPlayerState{
				OpaqueWords: p.originalOpaqueWords, HasOpaqueWords: true,
				Name: p.Name, HasName: true, ShirtIndex: p.Number - 1,
				X: int32(math.Round(p.Pos.X * originalFixedScale)), Y: int32(math.Round(p.Pos.Y * originalFixedScale)),
				Direction: OriginalDirectionIndex(p.Facing), Step: p.originalStep, State: p.originalState,
				Pace: originalPace(&p), PaceFixed: p.originalPaceFixed, Stamina: p.originalStaminaWord,
				Shooting: int(math.Round(p.Attributes.Shooting * 100)), Passing: int(math.Round(p.Attributes.Passing * 100)),
				AggressionThreshold: p.originalAggressionThreshold, Tackling: int(math.Round(p.Attributes.Tackling * 100)), Resilience: int(math.Round(p.Attributes.Resilience * 100)), HasDiscipline: true,
				FoulPoints: p.originalFoulPoints, Bookings: p.originalBookings, RecoveryLimit: p.originalRecoveryLimit, InjuryWord: p.originalInjuryWord, HasFoulContext: true,
				Substate: p.originalSubstate, GoalCount: p.originalGoalCount, HasGoalContext: true}
		}
	}
	snapshot.KickoffTeam, snapshot.HasKickoffTeam = m.originalKickoffTeam, true
	snapshot.CameraTarget, snapshot.HasCameraTarget = m.originalCameraTarget, true
	snapshot.CameraRetargeted = m.originalCameraRetargeted
	snapshot.CameraFixedX, snapshot.CameraFixedY, snapshot.HasFixedCamera = m.originalCameraFixedX, m.originalCameraFixedY, true
	snapshot.OfficialReady, snapshot.OfficialsEnabled, snapshot.HasOfficialContext = m.originalOfficialReady, m.originalOfficialsEnabled, true
	p := &m.originalOfficial
	snapshot.Official, snapshot.HasOfficial = OriginalOfficialState{
		GestureStarted: m.originalOfficialGestureStarted,
		X:              int32(math.Round(p.Pos.X * originalFixedScale)), Y: int32(math.Round(p.Pos.Y * originalFixedScale)),
		Direction: OriginalDirectionIndex(p.Facing), Step: p.originalStep, Pace: originalPace(p), State: p.originalState,
		DirectionHistory: p.originalDirectionHistory, AnimationPhase: p.AnimationPhase}, m.originalHasOfficial
	snapshot.KeeperHold, snapshot.KeeperDrift = m.keeperHold, m.originalKeeperGlobalDrift
	snapshot.KeeperAnimationCycles, snapshot.KeeperRepeatedCatch, snapshot.HasKeeperContext = m.originalKeeperAnimationCycles, m.originalKeeperRepeatedCatch, true
	snapshot.WindX, snapshot.WindY, snapshot.HasWind = m.originalWindX, m.originalWindY, true
	snapshot.Clock, snapshot.FatigueMask, snapshot.ClockRunning = m.originalClock, m.originalFatigueMask, m.originalClockRunning
	snapshot.RuleState, snapshot.RuleWait, snapshot.HasRuleState = m.originalRuleState, m.originalRuleWait, true
	snapshot.ThrowCountdown, snapshot.HasThrowCountdown = m.originalThrowCountdown, true
	snapshot.ThrowInTaker, snapshot.HasThrowInTaker = m.originalThrower, true
	snapshot.Scores, snapshot.HasScores = [2]int{m.Teams[0].Score, m.Teams[1].Score}, true
	snapshot.JoystickHistory, snapshot.HasJoystickHistory = m.originalDirectionInputHistory, true
	snapshot.GoalFrame, snapshot.HasGoalFrame = m.originalGoalFrame, true
	snapshot.Advertising, snapshot.HasAdvertising = m.originalAdvertising, true
	snapshot.BallDrawEnabled, snapshot.HasBallDrawFlag = m.originalBallDrawEnabled, true
	snapshot.ActorCameraX, snapshot.ActorCameraY, snapshot.HasActorCamera = m.originalCameraX, m.originalCameraY, true
	snapshot.CameraX, snapshot.CameraY, snapshot.HasCamera = m.originalDisplayCameraX, m.originalDisplayCameraY, true
	for team := range m.Teams {
		snapshot.Tactics[team] = OriginalTacticState{Slots: m.Teams[team].originalTacticSlots, Active: m.Teams[team].originalTacticSlot, Count: m.Teams[team].originalTacticCount}
		snapshot.HasTactics[team] = true
		snapshot.HasAssignedTacticData[team] = true
		for slot := 0; slot < 4; slot++ {
			snapshot.AssignedTacticData[team][slot] = append([]byte(nil), m.Config.AssignedTacticData[team][slot]...)
			snapshot.AssignedTacticNames[team][slot] = m.Config.AssignedTacticNames[team][slot]
		}
		for index, p := range m.Teams[team].Players {
			snapshot.Players[team][index] = OriginalPlayerState{
				X: int32(math.Round(p.Pos.X * originalFixedScale)), Y: int32(math.Round(p.Pos.Y * originalFixedScale)),
				Direction: OriginalDirectionIndex(p.Facing), Step: p.originalStep, Pace: originalPace(&p), State: p.originalState,
				BallDistance: p.originalBallDistance, TouchCooldown: p.touchCooldown, OwnerTicks: p.originalOwnerTicks,
				Shooting: int(math.Round(p.Attributes.Shooting * 100)), Passing: int(math.Round(p.Attributes.Passing * 100))}
			snapshot.Players[team][index].PaceFixed = p.originalPaceFixed
			snapshot.Players[team][index].Stamina = p.originalStaminaWord
			snapshot.Players[team][index].DirectionHistory = p.originalDirectionHistory
			snapshot.Players[team][index].AnimationPhase = p.AnimationPhase
			snapshot.Players[team][index].Substate = p.originalSubstate
			snapshot.Players[team][index].Visible, snapshot.Players[team][index].HasVisibility = p.originalVisible, true
			snapshot.Players[team][index].Targets, snapshot.Players[team][index].HasTargets = p.originalTargets, p.originalHasTargets
			if p.Keeper {
				snapshot.Players[team][index].KeeperTarget, snapshot.Players[team][index].HasKeeperTarget = p.originalKeeperTarget, true
			}
			snapshot.Players[team][index].AggressionThreshold = p.originalAggressionThreshold
			snapshot.Players[team][index].Tackling = int(math.Round(p.Attributes.Tackling * 100))
			snapshot.Players[team][index].Resilience = int(math.Round(p.Attributes.Resilience * 100))
			snapshot.Players[team][index].HasDiscipline = true
			snapshot.Players[team][index].GoalCount = p.originalGoalCount
			snapshot.Players[team][index].CelebrationWait = p.originalCelebrationWait
			snapshot.Players[team][index].HasGoalContext = true
			snapshot.Players[team][index].FoulPoints, snapshot.Players[team][index].Bookings = p.originalFoulPoints, p.originalBookings
			snapshot.Players[team][index].InjuryWord, snapshot.Players[team][index].RecoveryLimit = p.originalInjuryWord, p.originalRecoveryLimit
			snapshot.Players[team][index].ShirtIndex = p.Number - 1
			snapshot.Players[team][index].TackleHit, snapshot.Players[team][index].HasFoulContext = p.originalTackleHit, true
			snapshot.Players[team][index].Name, snapshot.Players[team][index].HasName = p.Name, true
			snapshot.Players[team][index].NameBytes, snapshot.Players[team][index].HasNameBytes = p.originalNameBytes, true
			snapshot.Players[team][index].EntryTarget, snapshot.Players[team][index].HasEntryTarget = p.originalEntryTarget, true
			snapshot.Players[team][index].OpaqueWords, snapshot.Players[team][index].HasOpaqueWords = p.originalOpaqueWords, true
			snapshot.Players[team][index].RestartTarget, snapshot.Players[team][index].HasRestartTarget = p.originalRestartTarget, true
		}
	}
	return snapshot
}

func (m *Match) originalNearest(team, exclude int) int {
	// D9BE includes fixed/partner actors in football nearest caches; restart
	// takers and NPC support use those caches independently of primary focus.
	return m.originalNearestEligible(team, exclude, false)
}

func (m *Match) originalPrimaryNearest(team, exclude int) int {
	// Automatic primary focus must leave the partner's bound actor available
	// to its own controller. This filter does not apply to restart selection.
	return m.originalNearestEligible(team, exclude, true)
}

func (m *Match) originalNearestEligible(team, exclude int, primaryOnly bool) int {
	best, distance := -1, int32(math.MaxInt32)
	for index := range m.Teams[team].Players {
		p := &m.Teams[team].Players[index]
		// D7EE dispatches shirt0 directly to138CA, bypassing the outfield
		// nearest/second-nearest accumulator atD820..D846.
		if index == 0 || index == exclude || p.SentOff || p.originalState == 0 || primaryOnly && m.secondaryBoundPlayer(team, index) || m.Config.Practice && team == 1 {
			continue
		}
		d := p.originalBallDistance
		if !p.originalDistanceReady {
			d = originalDistanceSquared(p.Pos, m.Ball.Pos)
		}
		// 0xd824/0xd82a use signed greater-than branches: an equal
		// cached distance replaces the earlier actor in the native scan.
		if d <= distance {
			best, distance = index, d
		}
	}
	return best
}

// selectOriginalHuman translates 0xc7d4. Its 24-unit gate applies to the current
// player's cached distance, and a distant player can retain control when the
// nearest teammate is in the runner's forward hemisphere.
func (m *Match) selectOriginalHuman(team int, input Input) {
	if m.originalFoulRestart.ControllerLocked {
		return
	}
	current := m.Teams[team].Controlled
	identity := m.Teams[team].ControlledActor(team)
	if identity >= 0 && m.originalRecordHasController(identity/11, current) {
		return // C7E8 retains any bound current record, including a foreign one.
	}
	if identity < 0 {
		current = 1
		m.Teams[team].selectOwnPlayer(current)
		identity = team*11 + current
	}
	best := m.originalPrimaryNearest(team, -1)
	if best < 0 {
		return
	}
	p := &m.Teams[identity/11].Players[current]
	if p.originalVisible && team*11+best == identity {
		return
	}
	directional := !p.originalVisible && !m.Teams[team].Players[best].originalVisible
	if input.Switch && !m.previous[team].Switch {
		exclude := -1
		if identity/11 == team {
			exclude = current
		}
		best = m.originalPrimaryNearest(team, exclude)
	} else if directional || p.originalVisible && p.originalBallDistance >= 576 && !input.Fire {
		bearing := OriginalDirectionIndex(m.Teams[team].Players[best].Pos.Sub(p.Pos))
		if x, y, known := m.originalSelectionMarkerOperands(team, p); known {
			// 14C1A leaves the partner mask-table offset in D0 and its low
			// mask word in D1. C820 consumes those actual marker operands;
			// neither a cooperative flag nor the nearest player's XY replaces them.
			bearing = originalDirectionIndexWords(x, y, int16(math.Floor(p.Pos.X)), int16(math.Floor(p.Pos.Y)))
		} else if !m.Config.Cooperative[team] {
			// 0x14bbc leaves D0 zero when there is no partner marker; D1
			// retains the primary marker's second mask word at0x19e4a.
			// The direction gate at0xc820 consumes those caller registers.
			markerX := (int(math.Floor(p.Pos.X)) - int(m.originalCameraX>>16) - 8) & 15
			low := [16]int{0, -32768, -16384, -8192, -4096, -2048, -1024, -512, -256, -128, -64, -32, -16, -8, -4, -2}[markerX]
			// SUB.W wraps the mask-coordinate difference through signed sixteen
			// bits; notably 0x8000 minus a positive pitch Y becomes positive.
			bearing = OriginalDirectionIndex(Vec2{float64(int16(-int(math.Floor(p.Pos.X)))), float64(int16(low - int(math.Floor(p.Pos.Y))))})
		}
		difference := (bearing - OriginalDirectionIndex(p.Facing)) & 7
		if !directional && (difference < 3 || difference > 5) {
			return
		}
		if input.Move.Len() > .1 {
			var candidates [8]int
			var distances [8]int32
			for i := range candidates {
				candidates[i], distances[i] = -1, math.MaxInt32
			}
			for i, q := range m.Teams[team].Players {
				// BBF6 skips inactive records before considering their cached
				// distance. Their old distance can otherwise win a direction.
				if q.SentOff || q.originalState == 0 || m.secondaryBoundPlayer(team, i) {
					continue
				}
				// 0xbbde feeds B8C8 the signed integer position words.
				// Fractional differences can cross its exact 2:1 boundary
				// and incorrectly remove the current actor from a candidate lane.
				direction := OriginalDirectionIndex(Vec2{math.Floor(m.Ball.Pos.X) - math.Floor(q.Pos.X), math.Floor(m.Ball.Pos.Y) - math.Floor(q.Pos.Y)})
				if q.originalBallDistance <= distances[direction] {
					candidates[direction], distances[direction] = i, q.originalBallDistance
				}
			}
			direction := OriginalDirectionIndex(input.Move)
			best = -1
			for _, candidate := range []int{direction, (direction + 1) & 7, (direction + 7) & 7} {
				if candidates[candidate] >= 0 {
					best = candidates[candidate]
					break
				}
			}
		}
	}
	if best < 1 || team*11+best == identity || m.originalBallOwner == team*11+best {
		return
	}
	m.Teams[team].selectOwnPlayer(best)
}

// RestoreOriginalPlayerState imports observed initial state once for an
// independent full-update comparison. It does not consume later observations.
func (m *Match) RestoreOriginalPlayerState(team, index int, state OriginalPlayerState) bool {
	if team < 0 || team > 1 || index < 0 || index > 10 {
		return false
	}
	p := &m.Teams[team].Players[index]
	restoreOriginalPlayerState(p, state)
	return true
}

func restoreOriginalPlayerState(p *Player, state OriginalPlayerState) {
	if state.HasKeeperTarget {
		p.originalKeeperTarget = state.KeeperTarget
	}
	if state.HasOpaqueWords {
		p.originalOpaqueWords = state.OpaqueWords
	}
	if state.HasRestartTarget {
		p.originalRestartTarget = state.RestartTarget
	}
	if state.HasEntryTarget {
		p.originalEntryTarget = state.EntryTarget
	}
	if state.HasName {
		p.Name = state.Name
	}
	if state.HasNameBytes {
		p.originalNameBytes = state.NameBytes
	} else {
		clear(p.originalNameBytes[:])
		for index, value := range []rune(p.Name) {
			if index >= 15 {
				break
			}
			p.originalNameBytes[index] = byte(value)
		}
	}
	if state.HasGoalContext {
		p.originalGoalCount, p.originalCelebrationWait = state.GoalCount, state.CelebrationWait
	}
	if state.HasFoulContext {
		p.originalFoulPoints, p.originalBookings, p.originalInjuryWord = state.FoulPoints, state.Bookings, state.InjuryWord
		p.originalRecoveryLimit, p.originalTackleHit = state.RecoveryLimit, state.TackleHit
		p.Number = state.ShirtIndex + 1
	}
	p.Pos = Vec2{float64(state.X) / originalFixedScale, float64(state.Y) / originalFixedScale}
	p.Facing = OriginalDirection(state.Direction)
	p.Vel = p.Facing.Mul(float64(state.Step) * 1000 * TPS / originalFixedScale)
	p.Attributes.Pace = clamp(float64((state.Pace-180)*4)/200, 0, 1)
	p.Attributes.Shooting, p.Attributes.Passing = float64(state.Shooting)/100, float64(state.Passing)/100
	p.originalStep, p.originalState = state.Step, state.State
	// A stopped actor can retain a valid hysteresis sector. Preserve the
	// observed word instead of deriving it from the independent step word.
	p.originalDirectionHistory = state.DirectionHistory
	p.originalPaceFixed = state.PaceFixed
	if p.originalPaceFixed == 0 {
		p.originalPaceFixed = uint32(state.Pace) << 16
	}
	p.originalStaminaWord = state.Stamina
	p.AnimationPhase, p.originalSubstate = state.AnimationPhase, state.Substate
	p.originalTargets, p.originalHasTargets = state.Targets, state.HasTargets
	if state.HasDiscipline {
		p.originalAggressionThreshold = state.AggressionThreshold
		p.Attributes.Tackling, p.Attributes.Resilience = float64(state.Tackling)/100, float64(state.Resilience)/100
	}
	p.originalBallDistance, p.originalDistanceReady = state.BallDistance, true
	p.touchCooldown, p.originalOwnerTicks = state.TouchCooldown, state.OwnerTicks
}

// SetOriginalBallOwner restores the team*11+player possession pointer, or -1.
// The source keeps this pointer while a dribbled ball is still travelling freely.
func (m *Match) SetOriginalBallOwner(owner int) {
	if owner < 0 || owner >= 22 {
		owner = -1
	}
	m.originalBallOwner = owner
	m.Ball.TrappedBy = -1
	if owner >= 0 {
		m.Ball.LastTeam, m.Ball.LastPlayer = owner/11, owner%11
		p := &m.Teams[owner/11].Players[owner%11]
		if p.originalState == 11 || p.Keeper && originalKeeperHoldingAction(p.originalState) {
			m.Ball.TrappedBy = owner
		}
	}
}

func originalDistanceSquared(a, b Vec2) int32 {
	x := int32(math.Floor(a.X)) - int32(math.Floor(b.X))
	y := int32(math.Floor(a.Y)) - int32(math.Floor(b.Y))
	return x*x + y*y
}

// originalBallFacing translates B91A's reverse bearing followed by a half turn.
// At coincident integer positions its zero-vector convention returns southwest;
// replacing it with the forward bearing silently selects northeast instead.
func (m *Match) originalBallFacing(p *Player) Vec2 {
	delta := Vec2{math.Floor(p.Pos.X) - math.Floor(m.Ball.Pos.X), math.Floor(p.Pos.Y) - math.Floor(m.Ball.Pos.Y)}
	return OriginalDirection(OriginalDirectionIndex(delta) + 4)
}

func originalPace(p *Player) int {
	if p.originalPaceFixed != 0 {
		return int(p.originalPaceFixed >> 16)
	}
	return 180 + int(byte(math.Round(clamp(p.Attributes.Pace, 0, 1)*200))>>2)
}

func (m *Match) originalFatigue(team, index int, p *Player) {
	// 14AB0 uses the record's shirt word. A replacement keeps the field slot
	// while carrying reserve shirt11/13, so its fatigue schedule also changes.
	if !m.originalClockRunning || (m.originalClock&uint32(m.originalFatigueMask))+1 != uint32(p.Number-1) {
		return
	}
	fixed := p.originalPaceFixed
	if fixed == 0 {
		fixed = uint32(originalPace(p)) << 16
	}
	factor := uint32((0xff37 + 2*p.originalStaminaWord) >> 1)
	value := (uint32(uint16(fixed>>8)) * factor) >> 7
	if value >= 60<<16 {
		p.originalPaceFixed = value
	}
}

func (m *Match) updateOriginalCamera() {
	x, y := m.originalCameraFocus()
	m.updateOriginalCameraTo(x, y)
}

func (m *Match) originalCameraFocus() (int32, int32) {
	return m.originalCameraFocusForTarget(m.originalCameraTarget)
}

func (m *Match) originalCameraFocusForTarget(target int) (int32, int32) {
	state := originalBallState(m.Ball, m.ballCurve)
	if target == -3 {
		state.X, state.Y = m.originalCameraHoldX, m.originalCameraHoldY
	}
	if target == -4 {
		state.X, state.Y = m.originalFoulRestart.CameraX, m.originalFoulRestart.CameraY
	}
	if target == -2 {
		state.X, state.Y = m.originalCameraFixedX, m.originalCameraFixedY
	}
	if target >= 0 && target < 22 {
		p := &m.Teams[target/11].Players[target%11]
		state.X, state.Y = int32(math.Round(p.Pos.X*originalFixedScale)), int32(math.Round(p.Pos.Y*originalFixedScale))
	}
	return state.X, state.Y
}

func (m *Match) updateOriginalCameraTo(x, y int32) {
	m.originalCameraX, m.originalCameraY = originalCameraStep(m.originalCameraX, m.originalCameraY, x, y)
}

// updateOriginalDisplayCamera retains the later video boundary separately
// from actor visibility/selection. The completed ball position feeds this
// callback; reusing the actor-stage offsets leaves rendering one update late.
func (m *Match) updateOriginalDisplayCamera() {
	x, y := m.originalCameraFocus()
	m.originalDisplayCameraX, m.originalDisplayCameraY = originalCameraStep(m.originalDisplayCameraX, m.originalDisplayCameraY, x, y)
}

// deferOriginalCameraTarget publishes a post-frame goal focus on the next view.
// Other actor and modal focus changes retain their immediate path.
func (m *Match) deferOriginalCameraTarget(target int) {
	if target == m.originalCameraTarget {
		return
	}
	if !m.originalCameraFocusDeferred {
		m.originalDeferredCameraTarget = m.originalCameraTarget
	}
	m.originalCameraTarget = target
	m.originalCameraFocusDeferred = true
}

func (m *Match) finishOriginalDisplayCamera() {
	if !m.originalCameraFocusDeferred {
		m.updateOriginalDisplayCamera()
		return
	}
	x, y := m.originalCameraFocusForTarget(m.originalDeferredCameraTarget)
	m.originalDisplayCameraX, m.originalDisplayCameraY = originalCameraStep(m.originalDisplayCameraX, m.originalDisplayCameraY, x, y)
	m.originalCameraFocusDeferred = false
	m.originalCameraRetargeted = true
}

// OriginalCameraStep exposes source11492 arithmetic for detached movie playback.
// Camera arguments are the saved display offsets; focus uses signed16.16 units.
func OriginalCameraStep(cameraX, cameraY, x, y int32) (int32, int32) {
	return originalCameraStep(cameraX, cameraY, x, y)
}

func originalCameraStep(cameraX, cameraY, x, y int32) (int32, int32) {
	groundX, groundY := cameraX+192*65536, cameraY+80*65536
	dx, dy := ((x+16*65536)-groundX)>>3, ((y-48*65536)-groundY)>>3
	limit := int32(16 * 65536)
	if dx > limit {
		dx = limit
	}
	if dx < -limit {
		dx = -limit
	}
	if dy > limit {
		dy = limit
	}
	if dy < -limit {
		dy = -limit
	}
	groundX += dx
	groundY += dy
	if groundX < 0 {
		groundX = 0
	}
	if groundX >= 960*65536 {
		groundX = 959 * 65536
	}
	if groundY < 0 {
		groundY = 0
	}
	if groundY >= 1312*65536 {
		groundY = 1311 * 65536
	}
	return groundX - 192*65536, groundY - 80*65536
}

func (m *Match) originalPlayerVisible(p *Player) bool {
	x := int(math.Floor(p.Pos.X)) - int(m.originalCameraX>>16) - 8
	y := int(math.Floor(p.Pos.Y)) - int(m.originalCameraY>>16) - 8
	// EEA0 clips the rendered sprite origin. AD26/AC92 and keeper poses
	// offset that origin vertically; using the ground center selects another
	// human one update early when a scissor pose touches the viewport edge.
	offset := -p.JumpHeight
	if _, specialY, present := p.OriginalSpecialSprite(); present {
		offset = float64(specialY)
	}
	y += int(offset)
	return x >= 0 && x < 335 && y >= 0 && y < 239
}

// originalTargetHeading preserves the sixteen-sector turning hysteresis in
// 0xb832. A one-sector change retains the existing eight-direction movement.
func originalTargetHeading(p *Player, target Vec2) Vec2 {
	delta := Vec2{math.Floor(target.X) - math.Floor(p.Pos.X), math.Floor(target.Y) - math.Floor(p.Pos.Y)}
	direction := OriginalDirectionIndex(delta)
	sectorBase := 0
	if math.Abs(delta.X) > math.Abs(delta.Y) {
		sectorBase = 1
	}
	if delta.X < 0 {
		sectorBase ^= 7
	}
	if delta.Y > 0 {
		sectorBase ^= 3
	}
	sector := 2*sectorBase + ((direction - sectorBase) & 7)
	previous := p.originalDirectionHistory
	if previous >= 0 {
		difference := sector - previous
		if difference >= 8 {
			difference = -difference
		}
		if difference&15 < 2 {
			return p.Facing
		}
	}
	p.originalDirectionHistory = sector
	return OriginalDirection(direction)
}

func (m *Match) originalChaseHeading(p *Player) Vec2 {
	delta := Vec2{math.Floor(m.Ball.Pos.X) - math.Floor(p.Pos.X), math.Floor(m.Ball.Pos.Y) - math.Floor(p.Pos.Y)}
	direction := OriginalDirectionIndex(delta)
	base := 0
	if math.Abs(delta.X) > math.Abs(delta.Y) {
		base = 1
	}
	if delta.X < 0 {
		base ^= 7
	}
	if delta.Y > 0 {
		base ^= 3
	}
	sector := 2*base + ((direction - base) & 7)
	// At 0xb8a0 D2 receives the complete sixteen-sector index. When
	// history is negative, 0xb8a6 skips the subtraction; 0xb990 tests
	// that index directly before applying the interception table.
	change := sector
	if p.originalDirectionHistory >= 0 {
		change = sector - p.originalDirectionHistory
		if change >= 8 {
			change = -change
		}
		change &= 15
	}
	facing := originalTargetHeading(p, m.Ball.Pos)
	state := originalBallState(m.Ball, m.ballCurve)
	vx, vy := state.VelocityX>>8, state.VelocityY>>8
	speed := int64(vx)*int64(vx) + int64(vy)*int64(vy)
	if m.originalBallDerivedReady {
		// B986 reads the integration-time scalar at A4-1A02. An earlier
		// actor's clearance must not change interception in this update.
		speed = int64(m.originalBallSpeedSquared)
	}
	if speed <= 30000 || change < 2 {
		return facing
	}
	// The 16x8 interception table at 0xb9ac is indexed by target sector and
	// the ball's eight-direction heading derived from integer velocity words.
	table := [16][8]int{
		{0, 0, 1, 1, 0, 0, 7, 0}, {1, 1, 1, 2, 1, 1, 0, 0}, {1, 1, 1, 2, 2, 1, 1, 0}, {1, 2, 2, 2, 3, 2, 2, 1},
		{1, 2, 2, 2, 3, 3, 2, 2}, {2, 2, 3, 3, 3, 4, 3, 3}, {3, 2, 3, 3, 3, 4, 4, 3}, {4, 3, 3, 4, 4, 4, 5, 4},
		{4, 4, 3, 4, 4, 4, 5, 5}, {5, 5, 4, 4, 5, 5, 5, 6}, {6, 5, 5, 4, 5, 5, 5, 6}, {7, 6, 6, 5, 5, 6, 6, 6},
		{7, 7, 6, 6, 5, 6, 6, 6}, {7, 0, 7, 7, 6, 6, 7, 7}, {7, 0, 0, 7, 7, 6, 7, 7}, {0, 0, 1, 0, 0, 7, 7, 0},
	}
	heading := OriginalDirectionIndex(Vec2{float64(state.VelocityX >> 16), float64(state.VelocityY >> 16)})
	if m.originalBallDerivedReady {
		// B99C similarly reads the saved heading word at A4-1A06.
		heading = m.originalBallHeading
	}
	return OriginalDirection(table[sector][heading])
}

func (m *Match) originalNudge(team, index, direction int) {
	p := &m.Teams[team].Players[index]
	state := originalBallState(m.Ball, 0)
	launch := OriginalDribbleState(direction, originalPace(p))
	state.VelocityX, state.VelocityY, state.Lift = launch.VelocityX, launch.VelocityY, launch.Lift
	m.Ball.applyOriginalState(state)
	m.Ball.LastTeam, m.Ball.LastPlayer = team, index
	// The human C164 caller at1030E records a new intentional ball launch.
	m.originalLastShooter = team*11 + index
	m.originalBallOwner = team*11 + index
	p.touchCooldown, p.originalOwnerTicks, p.lastTouch = 15, 300, m.Tick
	m.aftertouchTicks = 0
	m.aftertouchLateral, m.aftertouchVertical = 0, 0
	m.emit("touch", team, index, m.Ball.Pos)
}

func (m *Match) originalTrap(team, index int) {
	p := &m.Teams[team].Players[index]
	m.Ball.TrappedBy = team*11 + index
	m.originalBallOwner = m.Ball.TrappedBy
	m.Ball.Vel, m.Ball.VerticalSpeed = Vec2{}, 0
	m.Ball.LastTeam, m.Ball.LastPlayer = team, index
	p.originalState = 11
	p.lastTouch = m.Tick
	m.aftertouchTicks = 0
	m.aftertouchLateral, m.aftertouchVertical = 0, 0
	m.emit("trap", team, index, m.Ball.Pos)
}

func (m *Match) originalChip(team, index int, direction int) {
	p := &m.Teams[team].Players[index]
	// With pass assistance disabled, 0xc0fc uses 250+random%100.
	power := 250 + int(OriginalRandomWord(&m.originalRandomState)%100)
	state := originalBallState(m.Ball, 0)
	delta := OriginalDirection(direction).Mul(float64(power) * 1000)
	state.VelocityX, state.VelocityY, state.Lift = int32(math.Round(delta.X)), int32(math.Round(delta.Y)), 180000
	m.Ball.applyOriginalState(state)
	m.originalBallOwner = -1
	m.Ball.LastTeam, m.Ball.LastPlayer = team, index
	p.touchCooldown, p.lastTouch = 15, m.Tick
	m.emit("chip", team, index, m.Ball.Pos)
	m.originalLastShooter = team*11 + index
}

// originalHumanRun is the movement path after the input read at107A2.
// It preserves source coordinate fractions and the selected player's pace bonus.
func originalHumanRun(p *Player, moveDirection, pace int) (Vec2, int) {
	if moveDirection < 0 {
		return Vec2{}, 0
	}
	// 107B6..107E2 replaces only the coordinate's integer word.
	// Its left boundary deliberately returns the runner to positive40;
	// neutral input and the ordinary NPC runner bypass this branch.
	if math.Floor(p.Pos.X) <= -40 {
		p.Pos.X = originalCoordinateWord(p.Pos.X, 40)
	} else if math.Floor(p.Pos.X) >= 952 {
		p.Pos.X = originalCoordinateWord(p.Pos.X, 952)
	}
	if math.Floor(p.Pos.Y) <= -40 {
		p.Pos.Y = originalCoordinateWord(p.Pos.Y, -40)
	} else if math.Floor(p.Pos.Y) >= 1432 {
		p.Pos.Y = originalCoordinateWord(p.Pos.Y, 1432)
	}
	p.Facing = OriginalDirection(moveDirection)
	return p.Facing, pace + 20
}

// updateOriginalHuman translates the ordinary free-ball/owned-ball branches at
// 0x1005e..0x10328. Contacts use the integer distance cached after the previous
// player update. The return values are the direction vector and source step word.
func (m *Match) updateOriginalHuman(team, index int, in, previous Input) (Vec2, int) {
	p := &m.Teams[team].Players[index]
	if p.originalState == 44 {
		return m.originalShotActor(team, index)
	}
	p.originalDirectionHistory = -1
	if !p.originalDistanceReady {
		p.originalBallDistance = originalDistanceSquared(p.Pos, m.Ball.Pos)
		p.originalDistanceReady = true
	}
	pace, direction := originalPace(p), OriginalDirectionIndex(p.Facing)
	moveDirection := -1
	if in.Move.Len() > .05 {
		moveDirection = OriginalDirectionIndex(in.Move)
	}
	if m.originalBallOwner != team*11+index && p.originalState == 1 && int(m.Ball.Height) > 16 && moveDirection >= 0 {
		previousDirection := m.originalRetainedDirection(team, index)
		difference := moveDirection - previousDirection
		if previousDirection >= 0 && (difference == 4 || difference == -4) {
			p.originalState, p.originalDirectionHistory = 63, -1
			p.AnimationPhase = 0
			p.Facing = OriginalDirection(moveDirection)
			originalScissorStep(p)
			return p.Facing, p.originalStep
		}
	}
	run := func() (Vec2, int) {
		return originalHumanRun(p, moveDirection, pace)
	}

	if p.originalState == 11 && m.originalBallOwner != team*11+index {
		if m.Ball.TrappedBy == team*11+index {
			m.Ball.TrappedBy = -1
		}
		p.originalState = 1
		return p.Facing, p.originalStep
	}
	if owner := m.originalBallOwner; owner >= 0 && owner/11 != team && !m.Teams[owner/11].Players[owner%11].Keeper && originalDistanceSquared(p.Pos, m.Teams[owner/11].Players[owner%11].Pos) <= 144 {
		move, step := run()
		p.originalStep = step
		m.originalPlayerContact(team, index, owner)
		if p.originalStep == 0 {
			move = Vec2{}
		} else {
			move = p.Facing
		}
		return move, p.originalStep
	}
	// State 13 dispatches through updateOriginalJump before ordinary human
	// control. A retained portable replay countdown cannot select that action.
	if m.Ball.TrappedBy == team*11+index {
		if in.Fire || moveDirection >= 0 {
			m.pendingTrapInput[team] = false
		}
		if in.Fire {
			if moveDirection >= 0 {
				p.Facing = OriginalDirection(moveDirection)
			}
			m.Ball.Pos = p.Pos
			return Vec2{}, 0
		}
		if m.pendingTrapInput[team] && moveDirection < 0 {
			return Vec2{}, 0
		}
		m.pendingTrapInput[team] = false
		m.Ball.TrappedBy = -1
		if moveDirection < 0 {
			// 1037C ->103FC changes the action without clearing A0+4.
			// A neutral release ends the held pose while retaining possession.
			p.originalState = 1
			return p.Facing, p.originalStep
		}
		if receiver := m.originalTrapPassTarget(team, index); receiver >= 0 {
			m.originalPass(team, index, receiver)
			return Vec2{}, 0
		}
		// C474 sets state30 atC592. The caller at103DC retains ordinary
		// launch-frame movement and never establishes the state44 aftertouch
		// path used by the separate ordinary-shot caller at10266.
		p.Facing = OriginalDirection(moveDirection)
		state := originalBallState(m.Ball, m.ballCurve)
		launch := m.originalShot(p.Facing, p.Attributes.Shooting)
		state.VelocityX, state.VelocityY, state.Lift = launch.VelocityX, launch.VelocityY, launch.Lift
		m.Ball.applyOriginalState(state)
		m.originalBallOwner = -1
		p.originalState = 30
		m.Ball.LastTeam, m.Ball.LastPlayer, p.touchCooldown, p.lastTouch = team, index, 15, m.Tick
		m.originalLastShooter = team*11 + index
		m.emit("shot", team, index, m.Ball.Pos)
		return p.Facing, pace
	}
	owned := m.originalBallOwner == team*11+index
	if owned && p.touchCooldown > 0 {
		if in.Fire && p.originalBallDistance <= 256 {
			m.kick(team, index, p.Facing, 0, 0, "shot")
			*m.originalFireLatch(team, index) = true
			return Vec2{}, 0
		}
		if !in.Fire && moveDirection >= 0 {
			difference := (direction - moveDirection) & 7
			if difference > 4 {
				difference = 8 - difference
			}
			if difference >= 3 {
				m.originalChip(team, index, direction)
				return p.Facing, p.originalStep
			}
			if moveDirection == direction {
				return run()
			}
		} else {
			return run()
		}
	}
	near := p.originalBallDistance <= 64 && int(m.Ball.Height) <= 12
	if near && (owned || p.touchCooldown <= 0) {
		if !owned {
			// 0x1078a claims an incoming ball without manufacturing a nudge.
			m.originalBallOwner = team*11 + index
			m.Ball.LastTeam, m.Ball.LastPlayer = team, index
			p.originalOwnerTicks, p.lastTouch = 200, m.Tick
			if in.Fire {
				m.originalTrap(team, index)
			}
			return p.Facing, p.originalStep
		}
		p.originalOwnerTicks = 300
		if in.Fire {
			m.originalTrap(team, index)
			return Vec2{}, 0
		}
		if moveDirection < 0 {
			return Vec2{}, 0
		}
		p.Facing = OriginalDirection(moveDirection)
		m.originalNudge(team, index, moveDirection)
		return p.Facing, pace
	}
	if owned {
		p.originalOwnerTicks--
		if p.originalOwnerTicks < 0 {
			m.originalBallOwner = -1
		}
	}
	if in.Fire && !*m.originalFireLatch(team, index) && p.ActionTicks == 0 {
		if int(m.Ball.Height) > 16 {
			p.jumpTicks, p.ActionTicks, p.originalState = 44, 44, 13
			p.AnimationPhase = 128
			p.JumpHeight = OriginalJumpHeight(44)
			*m.originalFireLatch(team, index) = true
			m.emit("jump", team, index, p.Pos)
			return p.Facing, p.originalStep
		} else if m.originalBallOwner >= 0 && m.originalBallOwner/11 != team && m.originalBallOwner%11 != 0 {
			// 10166 starts action12 through AC2E without reading the
			// new joystick heading or using the legacy contact heuristic.
			p.originalState, p.originalStep, p.originalTackleHit = 12, pace*2, false
			p.tackleTicks, p.ActionTicks = 0, 0
			*m.originalFireLatch(team, index) = true
			m.emit("tackle", team, index, p.Pos)
			return p.Facing, p.originalStep
		}
	}
	return run()
}
