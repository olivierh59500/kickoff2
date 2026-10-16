package game

import (
	"math"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/olivierh59500/kickoff2/internal/engine"
)

const idleDemoDelay = 15 * time.Second

// idleInputActive includes held controls so a stationary finger or stick cannot
// start a demonstration underneath the player. Mouse motion also wakes it.
func (g *Game) idleInputActive() bool {
	x, y := ebiten.CursorPosition()
	moved := g.idleCursorReady && (x != g.idleCursorX || y != g.idleCursorY)
	g.idleCursorX, g.idleCursorY, g.idleCursorReady = x, y, true
	if moved || len(inpututil.AppendPressedKeys(nil)) != 0 || len(ebiten.AppendInputChars(nil)) != 0 || len(ebiten.AppendTouchIDs(nil)) != 0 {
		return true
	}
	// Android IMEs can commit text without a physical key or in-view touch.
	if field := g.activeTextField(); g.idleDemo == nil && field != nil {
		text := field.TextForRendering()
		start, end := field.Selection()
		changed := text != g.idleText || start != g.idleTextStart || end != g.idleTextEnd
		g.idleText, g.idleTextStart, g.idleTextEnd = text, start, end
		if changed {
			return true
		}
	}
	for button := ebiten.MouseButtonLeft; button <= ebiten.MouseButtonMax; button++ {
		if ebiten.IsMouseButtonPressed(button) {
			return true
		}
	}
	wheelX, wheelY := ebiten.Wheel()
	if wheelX != 0 || wheelY != 0 {
		return true
	}
	for _, pad := range ebiten.AppendGamepadIDs(nil) {
		if ebiten.IsStandardGamepadLayoutAvailable(pad) {
			for button := ebiten.StandardGamepadButton(0); button <= ebiten.StandardGamepadButtonMax; button++ {
				if ebiten.IsStandardGamepadButtonPressed(pad, button) {
					return true
				}
			}
			for axis := ebiten.StandardGamepadAxis(0); axis <= ebiten.StandardGamepadAxisMax; axis++ {
				if math.Abs(ebiten.StandardGamepadAxisValue(pad, axis)) > .25 {
					return true
				}
			}
		} else {
			for button := 0; button < ebiten.GamepadButtonCount(pad); button++ {
				if ebiten.IsGamepadButtonPressed(pad, ebiten.GamepadButton(button)) {
					return true
				}
			}
			// Unmapped triggers may rest at -1; only the primary stick is used.
			for axis := 0; axis < min(2, ebiten.GamepadAxisCount(pad)); axis++ {
				if math.Abs(ebiten.GamepadAxisValue(pad, axis)) > .25 {
					return true
				}
			}
		}
	}
	return false
}

func (g *Game) idleDemoEligible() bool {
	switch g.screen {
	case matchScreen:
		return g.Match != nil && (g.paused || g.Match.Phase == engine.HalfTime || g.Match.Phase == engine.FullTime)
	case replayScreen:
		return g.replayPaused
	default:
		return true
	}
}

// updateIdleDemo owns only the temporary presentation. Parent menus, replay
// buffers, tournament data, match input history and simulation remain untouched.
func (g *Game) updateIdleDemo(now time.Time, active bool) bool {
	if !g.idleLastUpdate.IsZero() && now.Sub(g.idleLastUpdate) > time.Second {
		g.idleSince = now
	}
	g.idleLastUpdate = now
	if g.idleRelease {
		g.idleSince = now
		if !active {
			g.idleRelease = false
		}
		return true // Consume the waking gesture through its release frame.
	}
	if g.idleDemo != nil {
		if active {
			g.stopIdleDemo()
			g.idleSince = now
			return true
		}
		g.advanceIdleDemo()
		return true
	}
	if active || !g.idleDemoEligible() {
		g.idleSince = now
		return false
	}
	if g.idleSince.IsZero() {
		g.idleSince = now
	}
	if now.Sub(g.idleSince) < idleDemoDelay {
		return false
	}
	field := g.activeTextField()
	g.idleTextFocused = field != nil && field.IsFocused()
	if g.idleTextFocused {
		field.Blur()
	}
	g.idleDemo = g.newIdleDemo()
	return true
}

func (g *Game) newIdleDemo() *Game {
	cfg := g.config
	cfg.Practice = false
	cfg.Human, cfg.Cooperative = [2]bool{}, [2]bool{}
	cfg.FixedPlayer, cfg.PartnerPlayer = [2]int{}, [2]int{}
	cfg.TeamKits, cfg.HasTeamKits = [2]engine.TournamentKit{}, [2]bool{}
	cfg.SquadBindings, cfg.HasSquadBindings = [2][13]uint8{}, [2]bool{}
	cfg.SquadBindingPorts = [2][3]uint8{}
	cfg.UnassignedSquadControllers, cfg.HasUnassignedSquadControllers = [2][64]uint8{}, [2]bool{}
	cfg.UserSquads, cfg.SelectedSquadIDs = [2][]byte{}, [2][13]uint32{}
	cfg.SelectedSquadRecords, cfg.HasSelectedSquadRecords = [2][13]uint8{}, [2]bool{}
	// Rotate every ordered country pairing before repeating. The parent owns
	// the cursor so waking a demo does not restart the country sequence.
	countries := len(engine.InternationalCountries)
	sequence := g.idleDemoSequence
	home := sequence % countries
	away := (home + 1 + sequence/countries) % countries
	g.idleDemoSequence = (sequence + 1) % (countries * (countries - 1))
	cfg.TeamNames = [2]string{engine.InternationalCountries[home], engine.InternationalCountries[away]}
	for team, name := range cfg.TeamNames {
		cfg.SquadFiles[team] = engine.InternationalSquad(name)
	}
	cfg.Seed ^= uint64(g.ticks + 1)
	match := engine.New(cfg)
	return &Game{Match: match, config: cfg, screen: matchScreen, scanner: true,
		images: map[string]*ebiten.Image{}, menus: g.menus, kits: originalKitDefaults(), kit: g.kit,
		camera: match.Ball.Pos, muted: g.muted, audio: g.audio}
}

func (g *Game) advanceIdleDemo() {
	demo := g.idleDemo
	demo.ticks++
	switch demo.Match.Phase {
	case engine.HalfTime:
		if !demo.Match.ContinueOriginalPeriod() {
			g.idleDemo = g.newIdleDemo()
		}
	case engine.FullTime:
		g.idleDemo = g.newIdleDemo()
	default:
		phase := demo.Match.Phase
		demo.Match.UpdatePALControllers([4]engine.Input{})
		demo.updateCamera()
		if demo.audio != nil && !demo.muted {
			demo.matchAudio(phase)
		}
	}
	// Demonstrations never append to the user's replay ring or movie recorder.
}

func (g *Game) stopIdleDemo() {
	if g.idleDemo == nil {
		return
	}
	g.idleDemo = nil
	g.idleSince, g.idleLastUpdate = time.Time{}, time.Time{}
	g.idleRelease = true
	if g.idleTextFocused {
		if field := g.activeTextField(); field != nil {
			field.Focus()
		}
		g.idleTextFocused = false
	}
	if g.audio != nil {
		g.audio.pause()
	}
}

func (g *Game) drawIdleDemo(frame *ebiten.Image) {
	demo := g.idleDemo
	demo.drawMatch(frame, demo.Match.Teams, demo.Match.Ball, demo.camera)
	label := "DEMO " + demo.Match.Teams[0].Name + " - " + demo.Match.Teams[1].Name
	if demo.ticks/200%2 != 0 {
		label = "DEMO - PRESS TO RETURN"
	}
	x := 8 + (demo.ticks/5)%(320-len(label)*6-16)
	demo.nativeText(frame, label, x, 4, gold, 0, 6, 8)
}
