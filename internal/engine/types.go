// Package engine implements the platform-independent football simulation.
// Coordinates use a downward Y axis and are measured in pitch units.
package engine

import "math"

const (
	TPS                = 50
	PitchWidth         = 912.0
	PitchHeight        = 1392.0
	GoalWidth          = 96.0
	GoalHeight         = 24.0
	PlayerRadius       = 7.0
	BallRadius         = 3.0
	PenaltyAreaWidth   = 528.0
	PenaltyAreaDepth   = 216.0
	PenaltySpotDepth   = 136.0
	GoalAreaWidth      = 240.0
	GoalAreaDepth      = 72.0
	CenterCircleRadius = 126.0
)

// Vec2 is a position, direction, or velocity in the pitch plane.
type Vec2 struct{ X, Y float64 }

func (v Vec2) Add(w Vec2) Vec2    { return Vec2{v.X + w.X, v.Y + w.Y} }
func (v Vec2) Sub(w Vec2) Vec2    { return Vec2{v.X - w.X, v.Y - w.Y} }
func (v Vec2) Mul(n float64) Vec2 { return Vec2{v.X * n, v.Y * n} }
func (v Vec2) Len() float64       { return math.Hypot(v.X, v.Y) }
func (v Vec2) Dot(w Vec2) float64 { return v.X*w.X + v.Y*w.Y }
func (v Vec2) Unit() Vec2 {
	n := v.Len()
	if n < 0.0001 {
		return Vec2{}
	}
	return v.Mul(1 / n)
}

// Formation identifies one of the four formations available during a match.
type Formation int

const (
	Formation424 Formation = iota
	Formation433
	Formation532
	Formation442
	FormationBalanced
	FormationBlitz
	FormationFalcon
	FormationLockout
	FormationZBreak
	FormationZXCross
	FormationCount
)

func (f Formation) String() string {
	return [...]string{"4-2-4", "4-3-3", "5-3-2", "4-4-2", "BALANCED", "BLITZ", "FALCON", "LOCKOUT", "ZBREAK", "ZXCROSS"}[clampInt(int(f), 0, int(FormationCount)-1)]
}

// Pitch describes the playing surface. These values are the saved native order;
// use OriginalIndex at original-format boundaries. Surface properties alter
// travel and bounce.
type Pitch int

const (
	Normal Pitch = iota
	Soggy
	Wet
	Artificial
	RandomPitch
)

func (p Pitch) String() string {
	return [...]string{"Normal", "Soggy", "Wet", "Artificial", "Random"}[clampInt(int(p), 0, 4)]
}

// Config contains match options and the original crowd-sound preference.
// Skill uses 0 for fourth division through 4 for international standard.
type Config struct {
	TeamKits    [2]TournamentKit
	HasTeamKits [2]bool
	// StartDownfield selects team zero's first-half end independently of the
	// match period. Each later half still swaps both teams' attack direction.
	StartDownfield bool
	Seed           uint64
	HalfSeconds    int
	Practice       bool
	// Competition keeps both interval choices as CONTINUE, as source 14B6.
	Competition bool
	// CrowdMuted follows the original C toggle; effects remain independent.
	CrowdMuted    bool
	Human         [2]bool
	FixedPlayer   [2]int
	Cooperative   [2]bool
	PartnerPlayer [2]int
	Formation     [2]Formation
	// OriginalTactics optionally supplies the original four-entry human bank.
	// Count zero retains the native Formation option as the first assigned entry.
	OriginalTactics [2]OriginalTacticState
	Skill           [2]int
	LeagueSkill     int
	// PacePercent zero selects the original normal/division-four setup rule.
	// Competitions can explicitly supply their source percentage for both sides.
	PacePercent [2]int
	Pitch       Pitch
	Wind        int
	Speed       float64
	Aftertouch  bool
	ExtraTime   bool
	TeamNames   [2]string
	SquadFiles  [2]string
	// UserSquads and UserTactics retain validated original data-file bytes.
	UserSquads  [2][]byte
	UserTactics [2][]byte
	// AssignedTacticData retains each custom TAC independently of the active slot.
	// UserTactics remains the active bytes and the legacy single-file setting.
	AssignedTacticData  [2][4][]byte
	AssignedTacticNames [2][4]string
	// SelectedSquadRecords contains eleven starters followed by reserves 12 and
	// 14. Each nonzero value is an original RecordIndex plus one, so zero can
	// represent a vacant shirt while records with a zero name seed remain valid.
	SelectedSquadRecords    [2][13]uint8
	HasSelectedSquadRecords [2]bool
	// SelectedSquadIDs mirrors the selected records' name-generator seeds for
	// settings written before record references became the canonical identity.
	SelectedSquadIDs [2][13]uint32
	// SquadBindings preserves setup bindings by shirt, including keeper/reserves.
	// Values are zero for none, one for primary, two for the fixed partner,
	// and three for the opposing primary input retained by original cup setup.
	// Absent bindings retain compatibility with FixedPlayer/PartnerPlayer settings.
	SquadBindings    [2][13]uint8
	HasSquadBindings [2]bool
	// SquadBindingPorts retains the joystick numbers represented by stored
	// primary/partner/opposing roles when a menu changes the player mode.
	// Zero primary means an older setting whose current context is inferred once.
	SquadBindingPorts [2][3]uint8
	// UnassignedSquadControllers retains imported physical joystick nibbles
	// by original pool-record index, including players without a shirt.
	UnassignedSquadControllers    [2][64]uint8
	HasUnassignedSquadControllers [2]bool
	Referee                       int
}

// DefaultConfig returns the original five-minute-half, one-player options.
func DefaultConfig() Config {
	return Config{Seed: 1990, HalfSeconds: 300, Human: [2]bool{true, false},
		Formation: [2]Formation{Formation424, Formation424}, Skill: [2]int{3, 3},
		// Boot5348 clears the aftertouch option;7906 maps it to flags19D56.
		Speed: 1, Aftertouch: false, TeamNames: [2]string{"Team A", "Team B"}, Referee: -1}
}

// Input is sampled once per simulation tick. Fire is the original single action
// button: its transitions determine trap, pass, shot, tackle, and header actions.
// A non-nil TacticSlot supplies a number-key command for the post-world poll.
type Input struct {
	Move   Vec2
	Fire   bool
	Switch bool
	// TacticSlot selects one of the four assigned entries after this world update.
	TacticSlot *int
	// CornerChoice selects the original panel: center, N, NE, E, SE, S, SW, W, NW.
	CornerChoice *int
}

// Attributes use normalized values in [0,1], decoded from original squad records.
type Attributes struct {
	Pace, Stamina, Aggression, Resilience, Passing, Shooting, Tackling float64
}

// Player exposes the visual state of an on-field player.
type Player struct {
	originalNameBytes                    [16]byte
	originalOpaqueWords                  [4]uint16
	originalFoulPoints, originalBookings uint16
	originalInjuryWord                   uint16
	originalEntryTarget                  Vec2
	originalGoalCount                    uint16
	originalCelebrationWait              int16
	originalAggressionThreshold          int
	originalTackleHit                    bool
	originalRestartTarget                Vec2
	originalTargets                      [20]Vec2
	originalHasTargets                   bool
	originalRecoveryLimit                uint16
	originalKeeperCarryCycles            int
	AnimationPhase                       uint16
	Name                                 string
	Number                               int
	Keeper                               bool
	Pos, Vel, Facing                     Vec2
	Stamina                              float64
	ActionTicks                          int
	JumpHeight                           float64
	YellowCards                          int
	SentOff                              bool
	Attributes                           Attributes
	// OriginalID is the source record's player-name seed, not a unique identity.
	OriginalID               uint32
	OriginalRecord           [42]byte
	Role                     int
	jumpTicks                int
	tackleTicks              int
	touchCooldown            int
	lastTouch                int64
	shotPauseTicks           int
	originalBallDistance     int32
	originalDistanceReady    bool
	originalOwnerTicks       int
	originalStep             int
	originalState            int
	originalKeeperTarget     Vec2
	originalDirectionHistory int
	originalPaceFixed        uint32
	originalStaminaWord      int
	originalVisible          bool
	originalSubstate         int
	originalKeeperDrift      int32
}

// Team contains eleven on-field players. Controlled is a zero-based shirt slot;
// ControlledOpposing retains a temporary selection in the other team's roster.
type Team struct {
	Name                string
	Players             [11]Player
	Controlled          int
	ControlledOpposing  bool
	Score               int
	Formation           Formation
	Substitutions       int
	Bench               [2]Player
	BenchUsed           [2]bool
	queuedFormation     Formation
	originalTacticSlots [4]int
	originalTacticSlot  int
	originalTacticCount int
}

// Ball remains independent of the dribbler. TrappedBy is -1 when it is free and
// otherwise encodes team*11+player. LastTeam and LastPlayer identify the last touch.
type Ball struct {
	Pos, Vel              Vec2
	Height, VerticalSpeed float64
	LastTeam, LastPlayer  int
	TrappedBy             int
}

type Phase int

const (
	Kickoff Phase = iota
	Playing
	SetPiece
	Goal
	HalfTime
	FullTime
	Shootout
)

func (p Phase) String() string {
	return [...]string{"KICK OFF", "PLAY", "SET PIECE", "GOAL", "HALF TIME", "FULL TIME", "PENALTIES"}[clampInt(int(p), 0, 6)]
}

type RestartKind int

const (
	KickOff RestartKind = iota
	ThrowIn
	Corner
	GoalKick
	FreeKick
	Penalty
)

func (r RestartKind) String() string {
	return [...]string{"KICK OFF", "THROW IN", "CORNER", "GOAL KICK", "FREE KICK", "PENALTY"}[clampInt(int(r), 0, 5)]
}

// Restart describes a stationary-ball restart. Hold Fire to charge its height and
// release to kick; ThrowIn and GoalKick also accept a quick press.
type Restart struct {
	PenaltyAimX, PenaltyAimStep int
	Kind                        RestartKind
	Team, Player                int
	Pos                         Vec2
	Countdown                   int
	ChargeTicks                 int
	Armed                       bool
	CornerChoice                int
}

// Event describes a notable action for sound effects, replay markers, and UI.
type Event struct {
	Tick         int64
	Kind         string
	Team, Player int
	Pos          Vec2
	// SoundSample selects a verified one-based source sample; zero uses the event default.
	SoundSample int
	// Sound preferences belong to the request instant, before a later keyboard
	// poll can change them. Unannotated external events use current preferences.
	SoundPractice, SoundCrowdMuted, HasSoundPreferences bool
}

// Match holds deterministic state. Hosts call UpdatePALControllers at TPS;
// Update advances an original game-loop iteration and may suspend at a selector.
// Reading its exported fields does not require a separate rendering API.
type Match struct {
	originalControlHandoffSuppressed               bool
	originalVideo                                  originalVideoState
	originalLinesman                               Player
	originalLinesmanFlag                           int
	originalHasLinesman                            bool
	originalActorCameraAtEntry                     bool
	originalPeriod                                 OriginalPeriodState
	originalPendingSubstitution                    OriginalPendingSubstitutionState
	originalSubstitutionDisplay                    OriginalPendingSubstitutionState
	originalKeyboardEvents                         []uint32
	originalFoulRestart                            OriginalFoulRestartState
	originalPeriodInjuryTicks                      uint32
	originalOfficialGestureStarted                 bool
	originalCameraHoldX, originalCameraHoldY       int32
	originalAppliedInjuryTicks                     uint32
	originalSubstituteInputs                       [4]Input
	originalSubstitutePolls                        [][4]Input
	originalHasSubstituteInputs                    bool
	originalSubstitution                           OriginalSubstitutionState
	originalSubstituteChoice                       int
	originalHasSubstituteChoice                    bool
	originalSubstitutionBlocked                    bool
	originalUpdateCompleted                        bool
	originalFoul                                   OriginalFoulState
	originalDisplayCameraX, originalDisplayCameraY int32
	originalOpeningSoundPending                    bool
	originalCameraFixedX, originalCameraFixedY     int32
	originalLastShooter                            int
	originalKickoffTeam                            int
	originalCameraTarget                           int
	originalDeferredCameraTarget                   int
	originalCameraFocusDeferred                    bool
	originalCameraRetargeted                       bool
	originalOfficialReady                          bool
	originalOfficialsEnabled                       bool
	originalMatchGoalPending                       bool
	originalOfficial                               Player
	originalHasOfficial                            bool
	originalQueuedSelectorInputs                   [4]Input
	originalQueuedModalPolls                       []Input
	originalQueuedModalController                  int
	originalHasQueuedSelectorInputs                bool
	originalDirectionInputHistory                  [4]int16
	originalBallSpeedSquared                       int32
	originalBallGoalGuide                          [3]int16
	originalBallHeading                            int
	originalBallDerivedReady                       bool
	originalWindX, originalWindY                   int32
	originalPALWait                                int
	originalPALHistorySampled                      bool
	originalPracticeGoalPending                    bool
	originalAdvertising                            OriginalAdvertisingState
	originalBallDrawEnabled                        bool
	originalGoalFrame                              OriginalGoalFrameState
	originalRuleState, originalRuleWait            int
	originalThrower, originalThrowCountdown        int
	originalCornerTeam, originalCornerTaker        int
	originalCornerCountdown, originalCornerHeld    int
	originalCornerSelecting                        bool
	originalCornerContinuation                     originalCornerContinuation
	originalKickoffReset                           bool
	originalFireConsumed                           [4]bool
	Config                                         Config
	Referee                                        Referee
	Teams                                          [2]Team
	Ball                                           Ball
	Phase                                          Phase
	Restart                                        Restart
	Tick                                           int64
	ClockTicks                                     int
	Half                                           int
	Message                                        string
	MessageTicks                                   int
	originalPublishedMessage                       string
	originalPersistentMessage                      string
	originalHUDPublished                           bool
	Events                                         []Event
	ShootoutScore                                  [2]int
	ShootoutTaken                                  [2]int
	Winner                                         int
	previous                                       [2]Input
	controls                                       [2]Input
	rng                                            uint64
	phaseTicks                                     int
	aftertouchTeam                                 int
	aftertouchTicks                                int
	keeperHold                                     int
	originalKeeperGlobalDrift                      int32
	originalKeeperAnimationCycles                  int
	originalKeeperRepeatedCatch                    int
	shootoutTeam                                   int
	timeExpired                                    bool
	ballPrevious                                   Vec2
	ballPreviousHeight                             float64
	penaltyFlightTicks                             int
	penaltyTeam                                    int
	pendingTrapInput                               [2]bool
	ballCurve                                      int32
	aftertouchLateral                              int
	aftertouchVertical                             int
	aftertouchHeading                              int
	aftertouchNewShot                              bool
	aftertouchPlayer                               int
	aftertouchFreeKick                             bool
	originalRandomState                            [3]uint16
	originalBallOwner                              int
	originalClock                                  uint32
	originalFatigueMask                            uint16
	originalClockRunning                           bool
	originalCameraX, originalCameraY               int32
	cooperativeInputs                              [2]Input
	cooperativePrevious                            [2]Input
	cooperativeFrame                               bool
}

func clamp(v, lo, hi float64) float64 { return math.Max(lo, math.Min(hi, v)) }
func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
