package game

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/olivierh59500/kickoff2/internal/engine"
	"math"
)

func pressed(keys ...ebiten.Key) bool {
	for _, key := range keys {
		if ebiten.IsKeyPressed(key) {
			return true
		}
	}
	return false
}
func axis(negative, positive bool) float64 {
	var n float64
	if negative {
		n--
	}
	if positive {
		n++
	}
	return n
}
func (g *Game) readKeyboardInput() [2]engine.Input {
	var in [2]engine.Input
	in[0].Move = engine.Vec2{X: axis(pressed(ebiten.KeyArrowLeft, ebiten.KeyA), pressed(ebiten.KeyArrowRight, ebiten.KeyD)), Y: axis(pressed(ebiten.KeyArrowUp, ebiten.KeyW), pressed(ebiten.KeyArrowDown, ebiten.KeyS))}
	cooperative := g.Match != nil && (g.Match.Config.Cooperative[0] || g.Match.Config.Cooperative[1])
	if cooperative {
		// WASD/Q belongs to the partner keyboard in cooperative games.
		// The numeric keypad remains available for Team B's original tactics.
		in[0].Move = engine.Vec2{X: axis(pressed(ebiten.KeyArrowLeft), pressed(ebiten.KeyArrowRight)), Y: axis(pressed(ebiten.KeyArrowUp), pressed(ebiten.KeyArrowDown))}
	}
	if g.Match != nil && g.Match.OriginalPendingSubstitution().Viewing {
		// Cursor keys belong to the source player view; WASD retains the
		// separately emulated joystick while that view is open.
		in[0].Move = engine.Vec2{X: axis(pressed(ebiten.KeyA), pressed(ebiten.KeyD)), Y: axis(pressed(ebiten.KeyW), pressed(ebiten.KeyS))}
		if cooperative {
			in[0].Move = engine.Vec2{}
		}
	}
	in[0].Fire = pressed(ebiten.KeyControlLeft, ebiten.KeyZ)
	in[0].Switch = inpututil.IsKeyJustPressed(ebiten.KeyTab)
	in[1].Move = engine.Vec2{X: axis(pressed(ebiten.KeyJ), pressed(ebiten.KeyL)), Y: axis(pressed(ebiten.KeyI), pressed(ebiten.KeyK))}
	in[1].Fire = pressed(ebiten.KeyControlRight, ebiten.KeyO)
	return in
}

// Tactic keys address teams globally, independently of joystick routing.
func (g *Game) readTacticKeys() {
	// The original shifted table produces symbols, not tactic digits, for
	// both number rows. Touch/gamepad selection supplies unshifted events.
	if pressed(ebiten.KeyShiftLeft, ebiten.KeyShiftRight, ebiten.KeyCapsLock) {
		return
	}
	for slot, key := range []ebiten.Key{ebiten.KeyDigit1, ebiten.KeyDigit2, ebiten.KeyDigit3, ebiten.KeyDigit4} {
		if inpututil.IsKeyJustPressed(key) {
			g.Match.QueueOriginalTacticSlot(0, slot)
		}
	}
	for slot, key := range []ebiten.Key{ebiten.KeyNumpad1, ebiten.KeyNumpad2, ebiten.KeyNumpad3, ebiten.KeyNumpad4} {
		if inpututil.IsKeyJustPressed(key) {
			g.Match.QueueOriginalTacticSlot(1, slot)
		}
	}
}

// ControllerSlots maps physical pads to active original controller slots. A
// multiplayer touch session reserves its first active slot for the touchscreen.
func ControllerSlots(config engine.Config, touch bool) []int {
	var slots []int
	for team, human := range config.Human {
		if human {
			slots = append(slots, team)
		}
	}
	for team, cooperative := range config.Cooperative {
		if cooperative && config.Human[team] {
			slots = append(slots, team+2)
		}
	}
	if touch && len(slots) > 1 {
		slots = slots[1:]
	}
	return slots
}

func mergeControllerInput(target *engine.Input, source engine.Input) {
	if source.Move.Len() > 0 {
		target.Move = source.Move
	}
	target.Fire = target.Fire || source.Fire
	target.Switch = target.Switch || source.Switch
	if source.TacticSlot != nil {
		target.TacticSlot = source.TacticSlot
	}
	if source.CornerChoice != nil {
		target.CornerChoice = source.CornerChoice
	}
}

// routeControllerInputs keeps each human's Fire edges separate, including the
// original second joystick joining Team A during two-player practice.
func routeControllerInputs(config engine.Config, touch bool, keyboard [4]engine.Input, pads []engine.Input, virtual engine.Input) [4]engine.Input {
	var result [4]engine.Input
	keyboardSlots := [4]int{0, 1, 2, 3}
	primary := 0
	if !config.Human[0] && config.Human[1] {
		primary, keyboardSlots[0] = 1, 1
		if config.Cooperative[1] {
			keyboardSlots[1], keyboardSlots[2] = 3, 3
		}
	} else if config.Human[0] && !config.Human[1] && config.Cooperative[0] {
		keyboardSlots[1] = 2
	}
	for source, input := range keyboard {
		mergeControllerInput(&result[keyboardSlots[source]], input)
	}
	slots := ControllerSlots(config, touch)
	for source, input := range pads {
		if source >= len(slots) {
			break
		}
		mergeControllerInput(&result[slots[source]], input)
	}
	if touch {
		mergeControllerInput(&result[primary], virtual)
	}
	return result
}

func readGamepadInput(id ebiten.GamepadID) engine.Input {
	var input engine.Input
	x := ebiten.StandardGamepadAxisValue(id, ebiten.StandardGamepadAxisLeftStickHorizontal)
	y := ebiten.StandardGamepadAxisValue(id, ebiten.StandardGamepadAxisLeftStickVertical)
	if math.Hypot(x, y) > 0.22 {
		input.Move = engine.Vec2{X: x, Y: y}
	}
	dx := axis(ebiten.IsStandardGamepadButtonPressed(id, ebiten.StandardGamepadButtonLeftLeft), ebiten.IsStandardGamepadButtonPressed(id, ebiten.StandardGamepadButtonLeftRight))
	dy := axis(ebiten.IsStandardGamepadButtonPressed(id, ebiten.StandardGamepadButtonLeftTop), ebiten.IsStandardGamepadButtonPressed(id, ebiten.StandardGamepadButtonLeftBottom))
	if dx != 0 || dy != 0 {
		input.Move = engine.Vec2{X: dx, Y: dy}
	}
	input.Fire = ebiten.IsStandardGamepadButtonPressed(id, ebiten.StandardGamepadButtonRightBottom)
	input.Switch = inpututil.IsStandardGamepadButtonJustPressed(id, ebiten.StandardGamepadButtonRightLeft)
	return input
}

func (g *Game) readControllerInput() [4]engine.Input {
	config := g.config
	if g.Match != nil {
		config = g.Match.Config
	}
	primary := g.readKeyboardInput()
	keyboard := [4]engine.Input{primary[0], primary[1]}
	if config.Cooperative[0] || config.Cooperative[1] {
		keyboard[2].Move = engine.Vec2{X: axis(pressed(ebiten.KeyA), pressed(ebiten.KeyD)), Y: axis(pressed(ebiten.KeyW), pressed(ebiten.KeyS))}
		keyboard[2].Fire = pressed(ebiten.KeyQ)
	}
	keyboard[3].Move = engine.Vec2{X: axis(pressed(ebiten.KeyB), pressed(ebiten.KeyComma)), Y: axis(pressed(ebiten.KeyU), pressed(ebiten.KeyN))}
	keyboard[3].Fire = pressed(ebiten.KeyY)
	g.padIDs = ebiten.AppendGamepadIDs(g.padIDs[:0])
	var pads []engine.Input
	slots := ControllerSlots(config, g.touch)
	for _, id := range g.padIDs {
		if ebiten.IsStandardGamepadLayoutAvailable(id) {
			input := readGamepadInput(id)
			if g.Match != nil && len(pads) < len(slots) {
				team := slots[len(pads)] % 2
				delta := 0
				if inpututil.IsStandardGamepadButtonJustPressed(id, ebiten.StandardGamepadButtonFrontTopLeft) {
					delta--
				}
				if inpututil.IsStandardGamepadButtonJustPressed(id, ebiten.StandardGamepadButtonFrontTopRight) {
					delta++
				}
				if delta != 0 {
					slot := (g.Match.OriginalTacticBank(team).Active + delta + 4) % 4
					input.TacticSlot = &slot
				}
			}
			pads = append(pads, input)
		}
	}
	in := routeControllerInputs(config, g.touch, keyboard, pads, engine.Input{Move: g.touchMove, Fire: g.touchFire})
	if g.Match != nil && (g.Match.Phase == engine.SetPiece || g.Match.OriginalCornerSelectorActive()) && g.Match.Restart.Kind == engine.Corner {
		slot := g.Match.Restart.Team
		if g.Match.Restart.Player == g.Match.PartnerPlayer(slot) {
			slot += 2
		}
		if !g.Match.Restart.Armed {
			g.cornerSelection = cornerChoice(in[slot].Move)
		}
		choice := g.cornerSelection
		in[slot].CornerChoice = &choice
	}
	return in
}

// readPointer supplies one activation edge for mouse and touch driven menus.
func (g *Game) readPointer() (int, int, bool) {
	x, y := ebiten.CursorPosition()
	clicked := inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft)
	touches := ebiten.AppendTouchIDs(nil)
	next := make(map[ebiten.TouchID]bool, len(touches))
	for _, id := range touches {
		next[id] = true
		if !g.previousTouches[id] && !g.suppressedTouches[id] {
			x, y = ebiten.TouchPosition(id)
			clicked = true
		}
	}
	for id := range g.suppressedTouches {
		if !next[id] {
			delete(g.suppressedTouches, id)
		}
	}
	g.previousTouches = next
	return x, y, clicked
}

// readTouchControls tracks the first left-gutter finger independently of Fire.
// The floating joystick begins at the contact point, so reaching it never moves
// the footballer abruptly. Released fingers always clear their action.
func (g *Game) readTouchControls() {
	g.touchIDs = ebiten.AppendTouchIDs(g.touchIDs[:0])
	points := make([]touchControlPoint, 0, len(g.touchIDs))
	for _, id := range g.touchIDs {
		x, y := ebiten.TouchPosition(id)
		points = append(points, touchControlPoint{id, x, y})
	}
	g.applyTouchControls(points)
}

// touchControlPoint is a sampled host contact, independent of player selection.
// Keeping this boundary explicit lets desktop checks exercise the same finger
// ownership, Fire release and controller routing used by the Android host.
type touchControlPoint struct {
	id   ebiten.TouchID
	x, y int
}

func (g *Game) applyTouchControls(points []touchControlPoint) {
	g.touchMove = engine.Vec2{}
	g.touchFire = false
	active := false
	for _, point := range points {
		id, x, y := point.id, point.x, point.y
		if g.suppressedTouches[id] {
			continue
		}
		if g.stickActive && id == g.stickID {
			active = true
			g.touchMove = engine.Vec2{X: (float64(x) - g.stickX) / 25, Y: (float64(y) - g.stickY) / 25}
			if g.touchMove.Len() > 1 {
				g.touchMove = g.touchMove.Unit()
			}
			if g.touchMove.Len() < 0.14 {
				g.touchMove = engine.Vec2{}
			}
			continue
		}
		if x < 80 && y > 68 && !g.stickActive {
			g.stickID = id
			g.stickActive = true
			active = true
			g.stickX = float64(x)
			g.stickY = float64(y)
		}
		if x >= 404 && y >= 159 && y < 239 {
			g.touchFire = true
		}
	}
	if !active {
		g.stickActive = false
	}
}

func (g *Game) fieldOffset() int {
	if g.touch {
		return 80
	}
	return 0
}
func (g *Game) pauseClick(x, y int) {
	x -= g.fieldOffset()
	if x < 72 || x > 248 {
		return
	}
	if y >= 108 && y < 132 {
		g.paused = false
		g.CancelInput()
	}
	if y >= 140 && y < 164 {
		g.Back()
	}
	if y >= 196 && y < 218 && len(g.replay) > 0 {
		if err := g.prepareRecordedPlayback(); err != nil {
			g.notice = err.Error()
			return
		}
		g.replayReturn = matchScreen
		g.replayIndex = 0
		g.replaySlow = false
		g.screen = replayScreen
		g.CancelInput()
	}
	if y >= 168 && y < 190 {
		g.substitutionTeam = 0
		if !g.Match.Config.Human[0] && g.Match.Config.Human[1] {
			g.substitutionTeam = 1
		}
		reserve := 0
		if x >= 160 {
			reserve = 1
		}
		if !g.Match.RequestOriginalSubstitution(g.substitutionTeam, reserve) {
			g.notice = "SUBSTITUTION NOT AVAILABLE"
		}
	}
}

// menuPadInput turns a held analogue direction into one navigation edge. It is
// independent of match input, so a controller can start and configure the game.
func (g *Game) menuPadInput() (dx, dy int, activate bool) {
	g.padIDs = ebiten.AppendGamepadIDs(g.padIDs[:0])
	if len(g.padIDs) == 0 {
		return
	}
	id := g.padIDs[0]
	return g.menuPadInputID(id)
}

func (g *Game) menuPadInputForSlot(config engine.Config, slot int) (dx, dy int, activate bool) {
	g.padIDs = ebiten.AppendGamepadIDs(g.padIDs[:0])
	slots := ControllerSlots(config, g.touch)
	index := 0
	for _, id := range g.padIDs {
		if !ebiten.IsStandardGamepadLayoutAvailable(id) {
			continue
		}
		if index < len(slots) && slots[index] == slot {
			input := readGamepadInput(id)
			g.prematchPadFireHeld = ebiten.IsStandardGamepadButtonPressed(id, ebiten.StandardGamepadButtonRightBottom)
			activate = inpututil.IsStandardGamepadButtonJustPressed(id, ebiten.StandardGamepadButtonRightBottom)
			direction := -1
			if input.Move.Len() > .1 {
				direction = engine.OriginalDirectionIndex(input.Move)
			}
			if direction != g.prematchPadDirection {
				g.prematchPadDirection = direction
				if g.prematchInputReady && direction >= 0 {
					move := engine.OriginalDirection(direction)
					if move.X < 0 {
						dx = -1
					} else if move.X > 0 {
						dx = 1
					}
					if move.Y < 0 {
						dy = -1
					} else if move.Y > 0 {
						dy = 1
					}
				}
			}
			return
		}
		index++
	}
	g.prematchPadDirection = -1
	g.prematchPadFireHeld = false
	return
}

func (g *Game) menuPadInputID(id ebiten.GamepadID) (dx, dy int, activate bool) {
	if !ebiten.IsStandardGamepadLayoutAvailable(id) {
		return
	}
	activate = inpututil.IsStandardGamepadButtonJustPressed(id, ebiten.StandardGamepadButtonRightBottom)
	if inpututil.IsStandardGamepadButtonJustPressed(id, ebiten.StandardGamepadButtonLeftLeft) {
		dx = -1
	}
	if inpututil.IsStandardGamepadButtonJustPressed(id, ebiten.StandardGamepadButtonLeftRight) {
		dx = 1
	}
	if inpututil.IsStandardGamepadButtonJustPressed(id, ebiten.StandardGamepadButtonLeftTop) {
		dy = -1
	}
	if inpututil.IsStandardGamepadButtonJustPressed(id, ebiten.StandardGamepadButtonLeftBottom) {
		dy = 1
	}
	x := ebiten.StandardGamepadAxisValue(id, ebiten.StandardGamepadAxisLeftStickHorizontal)
	y := ebiten.StandardGamepadAxisValue(id, ebiten.StandardGamepadAxisLeftStickVertical)
	direction := 0
	if math.Abs(x) > math.Abs(y) && math.Abs(x) > 0.6 {
		direction = 1
		if x < 0 {
			direction = 2
		}
	} else if math.Abs(y) > 0.6 {
		direction = 3
		if y < 0 {
			direction = 4
		}
	}
	if direction != g.padMenuDirection {
		switch direction {
		case 1:
			dx = 1
		case 2:
			dx = -1
		case 3:
			dy = 1
		case 4:
			dy = -1
		}
		g.padMenuDirection = direction
	}
	return
}

func (g *Game) padPausePressed() bool {
	g.padIDs = ebiten.AppendGamepadIDs(g.padIDs[:0])
	for _, id := range g.padIDs {
		if ebiten.IsStandardGamepadLayoutAvailable(id) && inpututil.IsStandardGamepadButtonJustPressed(id, ebiten.StandardGamepadButtonCenterRight) {
			return true
		}
	}
	return false
}
