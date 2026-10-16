// Package game connects the native football simulation to desktop and mobile input.
package game

import (
	"encoding/json"
	"fmt"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/olivierh59500/kickoff2/internal/autoplay"
	"github.com/olivierh59500/kickoff2/internal/engine"
	"github.com/olivierh59500/kickoff2/internal/importdata"
	"github.com/olivierh59500/kickoff2/internal/replaydata"
	"github.com/olivierh59500/kickoff2/internal/replayrender"
	"os"
	"path/filepath"
	"time"
)

type screen int

const (
	titleScreen screen = iota
	optionsScreen
	matchScreen
	helpScreen
	kitScreen
	competitionScreen
	replayScreen
	practiceScreen
	internationalScreen
	skillScreen
	controlsScreen
	tacticsScreen
	practiceSelectionScreen
	squadScreen
	preMatchScreen
	importScreen
	replayHomeScreen
	replayStoreScreen
	replayBrowserScreen
	replayEditScreen
)

// Game owns presentation state; the engine remains independent of Ebitengine.
type Game struct {
	competitionHumans             [8]bool
	competitionSavedConfig        *engine.Config
	competitionResult             *competitionResult
	competitionCupRepeated        bool
	competitionSquadController    int
	competitionEditor             *competitionTeamEditor
	competitionName               competitionNameEditor
	competitionSquadImportLocked  bool
	competitionLoad               bool
	prematchInputReady            bool
	prematchPadDirection          int
	prematchKeyDirection          int
	prematchPadFireHeld           bool
	prematch                      *engine.MatchPreparation
	toss                          engine.OriginalToss
	prematchStoreFields           *[6]string
	replayStoreReturn             screen
	idleDemo                      *Game
	idleDemoSequence              int
	idleSince, idleLastUpdate     time.Time
	idleRelease, idleTextFocused  bool
	idleCursorReady               bool
	idleCursorX, idleCursorY      int
	idleText                      string
	idleTextStart, idleTextEnd    int
	crowdTogglePending            bool
	periodMenuActive              bool
	periodMenuChoice              int
	periodMenuReleased            bool
	periodMenuFire                bool
	periodMenuDirection           int
	helpReturn                    screen
	substitutionReserve           int
	replayReturn                  screen
	replaySource                  *engine.Match
	suppressedTouches             map[ebiten.TouchID]bool
	menuActivate                  bool
	substitutionTeam              int
	substitutionIndex             int
	substituteScreen              bool
	padMenuDirection              int
	menus                         map[string]originalMenu
	optionTeam                    int
	tacticPending                 int
	tacticPendingData             []byte
	tacticPendingName             string
	Match                         *engine.Match
	config                        engine.Config
	screen                        screen
	selection                     int
	touch, paused, muted, scanner bool
	ticks, smoke                  int
	smokeInput                    bool
	capture                       string
	captured                      bool
	captureErr                    error
	dataDir, notice               string
	images                        map[string]*ebiten.Image
	camera                        engine.Vec2
	touchIDs                      []ebiten.TouchID
	previousTouches               map[ebiten.TouchID]bool
	stickID                       ebiten.TouchID
	stickActive                   bool
	stickX, stickY                float64
	touchMove                     engine.Vec2
	touchFire                     bool
	padIDs                        []ebiten.GamepadID
	audio                         *soundBank
	replay                        []snapshot
	replayStart                   int
	replayIndex                   int
	replaySlow                    bool
	kit                           [2]int
	tournament                    *engine.Tournament
	kits                          [10]kitDesign
	kitTeam                       int
	practiceMode                  int
	practicePlayers               int
	practiceDown                  bool
	setupPlayers                  int
	squadTeam                     int
	squadCursor                   int
	squadSlot                     int
	squadPending                  int
	squadSort                     int
	squadDirection                int
	squadRepeatWait               int
	squadControllerBits           uint8
	squadReturn                   screen
	skillReturn                   screen
	internationalTeam             int
	cornerSelection               int
	bot                           *autoplay.Controller
	botOptions                    autoplay.Options
	importDir, importKind         string
	importTeam                    int
	importReturn                  screen
	importFiles                   []importdata.File
	importPickerEnabled           bool
	importPickerRequested         bool
	replayCatalog                 replayCatalog
	replayLibraryIndex            int
	replayBrowserGolden           bool
	replayStoreFields             [6]string
	replayTyping                  bool
	replayText                    replayTextEditor
	replayEditClip                string
	goldenSelection               []string
	replayPlaylist                []string
	replayPlaylistIndex           int
	replaySourceKits              [10]kitDesign
	originalReplay                *replaydata.Clip
	liveRecorder                  *replaydata.Recorder
	recordedMovie                 *replaydata.Clip
	originalReplayFrames          []replaydata.Frame
	originalReplayCameras         []replayrender.Camera
	originalReplayCaptions        []*replaydata.Command
	originalReplayRenderer        *replayrender.Renderer
	originalReplayImage           *ebiten.Image
	indexedMatchRenderer          *replayrender.Renderer
	indexedMatchMetadata          replaydata.Metadata
	replayExportRequest           string
	documentExportOrigin          screen
	documentExportPending         bool
	replayStoreOriginal           bool
	replayPaused                  bool
}

// New creates the application with a deterministic original-style configuration.
func New() *Game {
	base, err := os.UserConfigDir()
	if err != nil {
		base = os.TempDir()
	}
	g := &Game{config: engine.DefaultConfig(), scanner: true, previousTouches: map[ebiten.TouchID]bool{}, suppressedTouches: map[ebiten.TouchID]bool{}, images: map[string]*ebiten.Image{}, dataDir: filepath.Join(base, "kickoff2"), kit: [2]int{0, 1}}
	g.menus = loadOriginalMenus()
	g.kits = originalKitDefaults()
	g.kitTeam = -1
	g.squadPending = -1
	g.practiceMode = -1
	g.loadSettings()
	g.practiceDown = g.config.StartDownfield
	return g
}

// SetTouchEnabled adds landscape controls without covering the football field.
func (g *Game) SetTouchEnabled(enabled bool) { g.touch = enabled; g.CancelInput() }

// SetMuted changes sound output while preserving the game state.
func (g *Game) SetMuted(muted bool) {
	g.muted = muted
	if g.audio != nil {
		g.audio.setMuted(muted)
	}
}

// SetAftertouch selects the original option for command-line sessions. Fresh
// settings keep the verified OFF boot default unless explicitly enabled.
func (g *Game) SetAftertouch(enabled bool) {
	g.config.Aftertouch = enabled
	if g.Match != nil {
		g.Match.Config.Aftertouch = enabled
	}
}

// SetAutoplay attaches a delayed visual observer to the first ordinary player
// controller. It advances only during running match updates and emits normal
// joystick/Fire inputs through the same engine API as a human controller.
func (g *Game) SetAutoplay(options autoplay.Options) {
	g.botOptions = options
	g.bot = autoplay.New(options)
}

// SetDataDir selects a private directory for settings and tournament saves.
func (g *Game) SetDataDir(dir string) {
	g.dataDir = dir
	g.config, g.kits, g.kit = engine.DefaultConfig(), originalKitDefaults(), [2]int{0, 1}
	g.muted = false
	g.loadSettings()
	g.practiceDown = g.config.StartDownfield
}

// SetSmoke enables deterministic input and an optional rendered frame capture.
func (g *Game) SetSmoke(ticks int, capture string) {
	g.smoke = ticks
	g.smokeInput = true
	g.capture = capture
	if ticks > 0 {
		g.config = engine.DefaultConfig()
	}
}

// SetSmokeInput permits bounded render/input checks with real host controls.
// Ordinary interactive games never receive the unattended probe's joystick.
func (g *Game) SetSmokeInput(enabled bool) { g.smokeInput = enabled }

// AtTitle lets the Android host apply its normal Back behavior at the main menu.
func (g *Game) AtTitle() bool { return g.idleDemo == nil && g.screen == titleScreen }

// Pause is idempotent so backgrounding never accidentally resumes a match.
func (g *Game) Pause() {
	g.stopIdleDemo()
	g.finishReplayTyping()
	g.finishCompetitionName()
	if g.screen == tacticsScreen || g.screen == importScreen && g.importReturn == tacticsScreen {
		g.captureCompetitionEditor()
	}
	if g.screen == matchScreen {
		g.paused = true
	}
	if g.screen == replayScreen {
		g.replayPaused = true
	}
	g.CancelInput()
	g.saveSettings()
	if g.audio != nil {
		g.audio.pause()
	}
}

// Back returns from a menu, or pauses an active match before leaving it.
func (g *Game) Back() {
	if g.idleDemo != nil {
		g.stopIdleDemo()
		return
	}
	if g.competitionName.active {
		g.finishCompetitionName()
		return
	}
	g.finishReplayTyping()
	if g.screen == importScreen {
		if g.competitionLoad {
			g.competitionLoad = false
			g.openFeatureMenu(competitionScreen, 3)
			return
		}
		if g.importReturn == competitionScreen && g.competitionEditor != nil {
			g.finishCompetitionEditor(false)
			return
		}
		if g.importReturn == tacticsScreen {
			g.openTactics(g.importTeam)
		} else {
			g.openFeatureMenu(g.importReturn, 0)
		}
		return
	}
	if g.screen == tacticsScreen {
		g.tacticPendingData, g.tacticPendingName = nil, ""
		if g.competitionEditor != nil {
			g.finishCompetitionEditor(true)
			return
		}
		g.screen, g.selection = optionsScreen, 0
		g.CancelInput()
		return
	}
	if g.screen == replayStoreScreen && g.replayStoreReturn == preMatchScreen {
		g.returnToPrematch()
		return
	}
	if g.screen == replayStoreScreen || g.screen == replayBrowserScreen || g.screen == replayEditScreen {
		g.replayTyping = false
		g.openReplayHome()
		return
	}
	if g.screen == replayScreen {
		g.replayPlaylist = nil
		g.screen = g.replayReturn
		g.CancelInput()
		return
	}
	if g.screen == helpScreen {
		g.screen = g.helpReturn
		g.CancelInput()
		return
	}
	if g.substituteScreen {
		g.substituteScreen = false
		g.CancelInput()
		return
	}
	if g.competitionSavedConfig != nil && (g.screen == squadScreen || g.screen == preMatchScreen) {
		g.captureCompetitionSetup()
		g.restoreCompetitionConfig()
		g.prematch = nil
		g.openCompetitionMenu()
		return
	}
	if g.screen == competitionScreen && g.competitionResult != nil {
		g.confirmCompetitionResult()
		return
	}
	if g.screen == matchScreen && g.tournament != nil && g.Match != nil && g.Match.Config.Competition && g.Match.Phase == engine.FullTime {
		g.finishTournamentMatch()
		return
	}
	if g.screen == matchScreen && g.Match != nil && g.Match.Phase == engine.FullTime {
		g.screen = titleScreen
		g.paused = false
		g.CancelInput()
		return
	}
	if g.screen == matchScreen && !g.paused {
		g.Pause()
		return
	}
	g.screen = titleScreen
	g.paused = false
	g.selection = 0
	g.substituteScreen = false
	g.CancelInput()
}

// CancelInput clears virtual controls on lifecycle and screen transitions.
func (g *Game) CancelInput() {
	g.idleSince, g.idleLastUpdate = time.Time{}, time.Time{}
	g.crowdTogglePending = false
	g.squadDirection, g.squadRepeatWait = -1, 0
	if g.Match != nil {
		g.Match.CancelInput()
	}
	g.stickActive = false
	g.touchMove = engine.Vec2{}
	g.touchFire = false
	if g.suppressedTouches == nil {
		g.suppressedTouches = map[ebiten.TouchID]bool{}
	}
	for id := range g.previousTouches {
		g.suppressedTouches[id] = true
	}
}

// Start begins skills practice, a human match, or a computer demonstration.
func (g *Game) Start(practice, demo bool) {
	cfg := g.config
	cfg.Practice = practice
	cfg.Human = [2]bool{!demo, cfg.Human[1]}
	if demo {
		cfg.Human = [2]bool{false, false}
	}
	g.startMatch(engine.New(cfg))
}

func (g *Game) startMatch(match *engine.Match) {
	g.restoreCompetitionConfig()
	g.Match = match
	g.prematch = nil
	g.prematchStoreFields = nil
	if g.bot != nil {
		g.bot = autoplay.New(g.botOptions)
	}
	g.screen = matchScreen
	g.substituteScreen = false
	g.periodMenuActive = false
	g.paused = false
	g.camera = g.Match.Ball.Pos
	g.replay = nil
	g.liveRecorder, g.recordedMovie = replaydata.NewRecorder(), nil
	g.clearOriginalReplay()
	g.replayStart = 0
	g.CancelInput()
}

// Layout retains the original pixel proportions and reserves both touch gutters.
func (g *Game) Layout(_, _ int) (int, int) {
	if g.touch {
		return 480, 256
	}
	return 320, 256
}

// Update samples controls once per PAL frame and advances the deterministic match.
func (g *Game) Update() error {
	if g.screen != replayStoreScreen && g.replayTyping {
		g.finishReplayTyping()
	}
	if g.audio == nil && !g.muted {
		var err error
		g.audio, err = newSoundBank()
		if err != nil {
			g.notice = err.Error()
			g.muted = true
		}
	}
	g.ticks++
	if g.smoke > 0 && g.ticks > g.smoke {
		if g.capture != "" && !g.captured {
			return nil
		}
		if g.captureErr != nil {
			return g.captureErr
		}
		return ebiten.Termination
	}
	// An unattended desktop window can still be visible without keyboard focus.
	if g.updateIdleDemo(time.Now(), ebiten.IsFocused() && g.idleInputActive()) {
		return nil
	}
	if !g.textInputActive() && (inpututil.IsKeyJustPressed(ebiten.KeyF11) || g.screen != matchScreen && inpututil.IsKeyJustPressed(ebiten.KeyF)) {
		ebiten.SetFullscreen(!ebiten.IsFullscreen())
	}
	if !g.textInputActive() && inpututil.IsKeyJustPressed(ebiten.KeyM) {
		g.SetMuted(!g.muted)
	}
	if !g.textInputActive() && inpututil.IsKeyJustPressed(ebiten.KeyC) {
		// The source compares decoded lowercase ASCII, including Shift/Caps.
		for _, decoded := range ebiten.AppendInputChars(nil) {
			g.queueCrowdKey(decoded)
		}
	}
	if !g.competitionName.active && inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		g.Back()
	}
	if !g.textInputActive() && inpututil.IsKeyJustPressed(ebiten.KeyH) {
		if g.screen == helpScreen {
			g.Back()
		} else {
			g.helpReturn = g.screen
			g.screen = helpScreen
			g.CancelInput()
			g.selection = 0
		}
	}
	g.menuActivate = false
	clickX, clickY, clicked := g.readPointer()
	if g.touch && clicked && g.screen == replayScreen && g.replayTouchAction(clickX, clickY) {
		clicked = false
	}
	if g.touch && clicked && clickX >= 400 {
		switch {
		case clickY >= 22 && clickY < 50:
			if g.screen == matchScreen {
				g.paused = !g.paused
				g.CancelInput()
			}
			if g.screen == replayScreen {
				g.replayPaused = !g.replayPaused
			}
			clicked = false
		case clickY >= 58 && clickY < 86:
			g.SetMuted(!g.muted)
			clicked = false
		case clickY >= 94 && clickY < 122:
			g.Back()
			clicked = false
		case clickY >= 126 && clickY < 154 && g.screen == replayScreen:
			g.replaySlow = !g.replaySlow
			clicked = false
		case clickY >= 126 && clickY < 146 && g.crowdControlAvailable():
			g.queueCrowdToggle()
			clicked = false
		case clickY >= 159 && clickY < 239 && g.screen != matchScreen:
			g.menuActivate = true
			clicked = false
		}
	}
	if g.screen != matchScreen {
		return g.updateMenu(clickX, clickY, clicked)
	}
	if g.Match.Phase == engine.HalfTime || g.Match.Phase == engine.FullTime {
		g.updatePeriodMenu(clickX, clickY, clicked)
		return nil
	}
	g.periodMenuActive = false
	if !g.paused {
		g.readTacticKeys()
		if g.touch && clicked && clickX >= 0 && clickX < 80 && clickY >= 0 && clickY < 68 {
			if slot, ok := touchTacticSlot(clickX, clickY); ok {
				g.Match.QueueOriginalTacticSlot(touchPrimaryTeam(g.Match.Config), slot)
				clicked = false
			}
		}
	}
	_, _, selectingSubstitute := g.Match.OriginalSubstitutionSelection()
	if inpututil.IsKeyJustPressed(ebiten.KeyP) || !selectingSubstitute && inpututil.IsKeyJustPressed(ebiten.KeySpace) || g.padPausePressed() {
		g.paused = !g.paused
		g.CancelInput()
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyX) {
		g.scanner = !g.scanner
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyR) && g.Match.GoalReplayAvailable() && len(g.replay) > 0 {
		if err := g.prepareRecordedPlayback(); err != nil {
			g.notice = err.Error()
			return nil
		}
		g.replayIndex = 0
		g.replaySlow = false
		g.replayReturn = matchScreen
		g.screen = replayScreen
		return nil
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyS) && g.Match.GoalReplayAvailable() && len(g.replay) > 0 {
		if err := g.prepareRecordedPlayback(); err != nil {
			g.notice = err.Error()
			return nil
		}
		g.replayIndex = 0
		g.replaySlow = true
		g.replayReturn = matchScreen
		g.screen = replayScreen
		return nil
	}
	if g.touch {
		g.readTouchControls()
	}
	g.updateInjuryReserveSelection(clickX, clickY, clicked)
	g.updateOriginalSubstitutionInput(clickX, clickY, clicked)

	if g.paused {
		if clicked {
			g.pauseClick(clickX, clickY)
		}
		return nil
	}
	inputs := g.readControllerInput()
	if g.smoke > 0 && g.smokeInput {
		inputs[0] = engine.Input{Move: engine.Vec2{X: 0.22, Y: -1}, Fire: g.ticks%70 < 8}
	}
	if g.bot != nil {
		inputs[0] = g.bot.Update(autoplay.ObserveMatch(g.Match, 0))
	}
	oldPhase := g.Match.Phase
	advanced := g.Match.UpdatePALControllers(inputs)
	g.updateCamera()
	if advanced && g.Match.Phase != engine.FullTime {
		g.recordFrame()
	}
	if g.audio != nil {
		g.matchAudio(oldPhase)
	}
	if advanced {
		g.applyCrowdToggle()
	}
	return nil
}

// Close releases active audio players and writes the user's latest preferences.
func (g *Game) Close() {
	g.finishReplayTyping()
	g.clearOriginalReplay()
	g.saveSettings()
	if g.audio != nil {
		g.audio.close()
	}
}

// Report returns a compact smoke-test result from the actual running match.
func (g *Game) Report() string {
	if g.idleDemo != nil {
		match := g.idleDemo.Match
		return fmt.Sprintf("idle-demo teams=%s/%s humans=%v score=%d:%d ball=(%.2f,%.2f)",
			match.Teams[0].Name, match.Teams[1].Name, match.Config.Human,
			match.Teams[0].Score, match.Teams[1].Score, match.Ball.Pos.X, match.Ball.Pos.Y)
	}
	if g.Match == nil {
		return "no match"
	}
	return fmt.Sprintf("updates=%d score=%d:%d ball=(%.2f,%.2f) phase=%v capture=%t", g.ticks-1, g.Match.Teams[0].Score, g.Match.Teams[1].Score, g.Match.Ball.Pos.X, g.Match.Ball.Pos.Y, g.Match.Phase, g.captured)
}

type settings struct {
	Config engine.Config
	Kit    [2]int
	Muted  bool
	Kits   *[10]kitDesign `json:"Kits,omitempty"`
}

func (g *Game) loadSettings() {
	data, err := os.ReadFile(filepath.Join(g.dataDir, "settings.json"))
	if err != nil {
		return
	}
	var saved settings
	if json.Unmarshal(data, &saved) == nil {
		if saved.Config.HalfSeconds >= 30 && saved.Config.HalfSeconds <= 1200 {
			g.config = saved.Config
			g.kit = saved.Kit
			g.muted = saved.Muted
			if saved.Kits != nil {
				valid := true
				for _, design := range saved.Kits {
					valid = valid && design.Style >= 0 && design.Style < 6 && design.Shirt >= 0 && design.Shirt < len(kitColors) && design.Shorts >= 0 && design.Shorts < len(kitColors)
				}
				if valid {
					g.kits = *saved.Kits
				}
			}
		}
	}
}
func (g *Game) saveSettings() {
	if err := os.MkdirAll(g.dataDir, 0755); err != nil {
		g.notice = err.Error()
		return
	}
	config := g.config
	if g.competitionSavedConfig != nil {
		config = *g.competitionSavedConfig
	} else if g.competitionEditor != nil {
		config = g.competitionEditor.saved
	}
	data, err := json.MarshalIndent(settings{Config: config, Kit: g.kit, Muted: g.muted, Kits: &g.kits}, "", "  ")
	if err == nil {
		err = os.WriteFile(filepath.Join(g.dataDir, "settings.json.tmp"), data, 0644)
	}
	if err == nil {
		err = os.Rename(filepath.Join(g.dataDir, "settings.json.tmp"), filepath.Join(g.dataDir, "settings.json"))
	}
	if err != nil {
		g.notice = err.Error()
	}
}

// ShowScreen opens a named menu for reproducible presentation checks.
func (g *Game) ShowScreen(name string) error {
	switch name {
	case "menu":
		g.screen = titleScreen
	case "options":
		g.screen = optionsScreen
	case "tactics":
		g.openTactics(0)
	case "kit":
		g.screen = kitScreen
		g.kitTeam = -1
	case "help":
		g.screen = helpScreen
		g.helpReturn = titleScreen
	case "practice":
		g.openOriginalPractice()
	case "penalties":
		g.Start(false, false)
		g.Match.StartShootout()
	case "halftime":
		g.Start(false, false)
		// Presentation fixture only; the natural reference checks period entry.
		g.Match.Phase = engine.HalfTime
	case "setup":
		g.screen = controlsScreen
	case "international":
		g.screen = internationalScreen
	case "squad":
		g.openSquad(0, preMatchScreen)
	case "prematch":
		g.openPrematch(nil)
	case "league":
		g.beginCompetition(false)
	case "cup":
		g.beginCompetition(true)
	case "four-player":
		g.config.Human = [2]bool{true, true}
		g.config.Cooperative = [2]bool{true, true}
		// This direct CLI preview bypasses the original squad menus. Supply
		// its explicit legal pair instead of choosing partners inside a menu.
		for team := 0; team < 2; team++ {
			g.config.HasSquadBindings[team] = true
			g.config.SquadBindings[team] = [13]uint8{1: 1, 4: 2}
			g.config.SquadBindingPorts[team] = [3]uint8{}
			g.config.UnassignedSquadControllers[team], g.config.HasUnassignedSquadControllers[team] = [64]uint8{}, false
		}
		g.initializePartners()
		g.Start(false, false)
	case "corner":
		g.Start(true, false)
		g.Match.PracticeRestart(engine.Corner)
		g.camera = g.Match.Ball.Pos
		g.updateCamera()
		g.paused = true
	case "replays":
		g.openReplayHome()
	case "replay-editor":
		g.loadReplayCatalog()
		g.screen = replayEditScreen
	case "imports":
		g.openImport("sqd", 0, optionsScreen)
	default:
		return fmt.Errorf("unknown screen %q", name)
	}
	if g.screen != squadScreen {
		g.selection = 0
	}
	if source := g.featureMenuSource(); source != "" {
		codes := g.featureMenuCodes()
		if len(codes) > 0 {
			g.selection = codes[0]
		}
	}
	if g.screen == kitScreen {
		g.selection = 20
	}
	if g.screen == competitionScreen {
		g.selection = 1
	}
	if g.screen == preMatchScreen {
		g.selection = g.prematchSelection()
	}
	if g.screen == replayHomeScreen {
		g.selection = 1
	}
	if g.screen == replayEditScreen || g.screen == replayBrowserScreen {
		g.selection = 2
	}
	return nil
}
