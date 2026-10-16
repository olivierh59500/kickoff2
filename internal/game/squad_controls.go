package game

import (
	"fmt"
	"sort"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/olivierh59500/kickoff2/internal/engine"
)

func (g *Game) hasSquadBinding(role uint8) bool {
	for _, binding := range g.config.SquadBindings[g.squadTeam] {
		if binding == role {
			return true
		}
	}
	return false
}

func (g *Game) resetSquadBindings(team int) {
	g.config.HasSquadBindings[team] = false
	g.config.SquadBindings[team] = [13]uint8{}
	g.config.SquadBindingPorts[team] = [3]uint8{}
	g.config.UnassignedSquadControllers[team] = [64]uint8{}
	g.config.HasUnassignedSquadControllers[team] = false
	g.config.FixedPlayer[team], g.config.PartnerPlayer[team] = 0, 0
}

func originalSquadControllersValid(mode int, mask uint8) bool {
	switch mode {
	case 0, 2:
		return true
	case 1:
		return mask&1 != 0 || mask&2 == 0
	case 3:
		return mask&5 == 5
	case 4:
		return mask&10 == 10
	}
	return false
}

func (g *Game) originalSquadMode() int {
	if g.practiceMode >= 0 && g.practicePlayers == 2 && g.squadTeam == 0 {
		return 1
	}
	primary := g.squadControllerNumber(false)
	if g.config.Cooperative[g.squadTeam] {
		return primary + 2
	}
	return (primary - 1) * 2
}

func (g *Game) rebuildSquadControllerMask() {
	var mask uint8
	for _, controller := range g.config.UnassignedSquadControllers[g.squadTeam] {
		if controller >= 1 && controller <= 4 {
			mask |= 1 << (controller - 1)
		}
	}
	for _, role := range g.config.SquadBindings[g.squadTeam] {
		controller := 0
		switch role {
		case 1:
			controller = g.squadControllerNumber(false)
		case 2:
			controller = g.squadControllerNumber(true)
		case 3:
			controller = 1
		}
		if controller >= 1 && controller <= 4 {
			mask |= 1 << (controller - 1)
		}
	}
	g.squadControllerBits = mask
}

// The original squad screen builds this word once on entry. Controller-button
// edits then update it independently of the packed player records.
func (g *Game) squadControllerMask() uint8 {
	return g.squadControllerBits
}

// Existing settings use outfield indices. Materialize them once; new setup
// bindings also retain the keeper/reserve cases representable by source bytes.
func (g *Game) initializeSquadBindings(pool []engine.SquadPlayer) {
	team := g.squadTeam
	if g.config.HasSquadBindings[team] {
		g.normalizeSquadBindingPorts()
		g.initializeUnassignedSquadControllers(pool)
		return
	}
	ports := g.config.SquadBindingPorts[team]
	if ports[0] == 0 {
		ports = g.currentSquadBindingPorts(team)
	}
	fixed, partner := g.config.FixedPlayer[team], g.config.PartnerPlayer[team]
	g.config.SquadBindings[team] = [13]uint8{}
	for _, player := range pool {
		if slot := g.selectedSquadRecordSlot(team, player.RecordIndex); slot >= 0 {
			g.config.SquadBindings[team][slot] = g.squadRoleForController(player.Record[0x15] >> 4)
		}
	}
	for index, slot := range [2]int{fixed, partner} {
		role := g.squadRoleForController(ports[index])
		if role != 0 && slot > 0 && slot < 11 && !g.hasSquadBinding(role) {
			g.config.SquadBindings[team][slot] = role
		}
	}
	g.config.HasSquadBindings[team] = true
	g.config.SquadBindingPorts[team] = g.currentSquadBindingPorts(team)
	g.syncSquadBindingSlots()
	g.initializeUnassignedSquadControllers(pool)
}

func (g *Game) syncSquadBindingSlots() {
	g.syncSquadBindingSlotsForTeam(g.squadTeam)
}

func (g *Game) syncSquadBindingSlotsForTeam(team int) {
	g.config.FixedPlayer[team], g.config.PartnerPlayer[team] = 0, 0
	for slot := 1; slot < 11; slot++ {
		switch g.config.SquadBindings[team][slot] {
		case 1:
			g.config.FixedPlayer[team] = slot
		case 2:
			g.config.PartnerPlayer[team] = slot
		}
	}
}

// The native controls shortcut also edits an already materialized squad.
func (g *Game) changeFixedPlayer(team, delta int) {
	slot := (g.config.FixedPlayer[team] + delta + 11) % 11
	g.config.FixedPlayer[team] = slot
	if !g.config.HasSquadBindings[team] {
		return
	}
	ports := configuredSquadBindingPorts(g.config, team)
	if g.config.SquadBindingPorts[team][0] == 0 {
		g.config.SquadBindingPorts[team] = ports
	}
	g.config.SquadBindingPorts[team][0] = ports[0]
	for record, controller := range g.config.UnassignedSquadControllers[team] {
		if controller == ports[0] {
			g.config.UnassignedSquadControllers[team][record] = 0
		}
	}
	for index, role := range g.config.SquadBindings[team] {
		if role == 1 {
			g.config.SquadBindings[team][index] = 0
		}
	}
	if slot > 0 {
		g.config.SquadBindings[team][slot] = 1
	}
	g.syncSquadBindingSlotsForTeam(team)
}

func (g *Game) squadPadInput() (dx, dy int, fire bool) {
	g.padIDs = ebiten.AppendGamepadIDs(g.padIDs[:0])
	slots, index := ControllerSlots(g.config, g.touch), 0
	target := g.squadTeam
	if g.competitionSquadController == 1 && g.squadTeam == 1 {
		target = 0
	}
	id := ebiten.GamepadID(-1)
	for _, candidate := range g.padIDs {
		if !ebiten.IsStandardGamepadLayoutAvailable(candidate) {
			continue
		}
		if index < len(slots) && slots[index] == target {
			id = candidate
			break
		}
		index++
	}
	if id < 0 {
		return
	}
	fire = inpututil.IsStandardGamepadButtonJustPressed(id, ebiten.StandardGamepadButtonRightBottom)
	x := axis(ebiten.IsStandardGamepadButtonPressed(id, ebiten.StandardGamepadButtonLeftLeft), ebiten.IsStandardGamepadButtonPressed(id, ebiten.StandardGamepadButtonLeftRight))
	y := axis(ebiten.IsStandardGamepadButtonPressed(id, ebiten.StandardGamepadButtonLeftTop), ebiten.IsStandardGamepadButtonPressed(id, ebiten.StandardGamepadButtonLeftBottom))
	if x == 0 && y == 0 {
		x = ebiten.StandardGamepadAxisValue(id, ebiten.StandardGamepadAxisLeftStickHorizontal)
		y = ebiten.StandardGamepadAxisValue(id, ebiten.StandardGamepadAxisLeftStickVertical)
		if x*x+y*y < .36 {
			return
		}
	}
	move := engine.OriginalDirection(engine.OriginalDirectionIndex(engine.Vec2{X: x, Y: y}))
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
	return
}

// The source reacts immediately to a new direction, then repeats after 30
// menu polls and every 5 thereafter. Neutral input starts a fresh gesture.
func (g *Game) stepSquadDirection(dx, dy, count int) {
	if dx == 0 && dy == 0 {
		g.squadDirection, g.squadRepeatWait = -1, 0
		return
	}
	direction := engine.OriginalDirectionIndex(engine.Vec2{X: float64(dx), Y: float64(dy)})
	if direction != g.squadDirection {
		g.squadDirection, g.squadRepeatWait = direction, 30
	} else {
		g.squadRepeatWait--
		if g.squadRepeatWait > 0 {
			return
		}
		g.squadRepeatWait = 5
	}
	g.moveSquadCursor(dx, dy, count)
}

// moveSquadCursor follows 7356..7448. Minus one is DONE; the source enters
// the last player with Up, returns to DONE with Down, and skips shirt 13.
func (g *Game) moveSquadCursor(dx, dy, count int) {
	if count < 1 || dx != 0 && dy != 0 {
		return
	}
	if dx != 0 || dy != 0 {
		g.notice = ""
	}
	if dy < 0 {
		if g.squadCursor < 0 {
			g.squadCursor = count - 1
		} else if g.squadCursor > 0 {
			g.squadCursor--
		}
	} else if dy > 0 && g.squadCursor >= 0 {
		g.squadCursor++
		if g.squadCursor >= count {
			g.squadCursor = -1
		}
	}
	if dy != 0 {
		g.squadPending = g.squadCursor
	}
	column := g.selection - 12
	if dx < 0 {
		switch column {
		case 0:
		case 13:
			column = 11
		case 18:
			column = 13
		case 19:
			column = 18
		default:
			column--
		}
	} else if dx > 0 {
		switch column {
		case 11:
			column = 13
		case 13:
			column = 18
		case 18:
			if g.config.Cooperative[g.squadTeam] {
				column = 19
			}
		case 19:
		default:
			column++
		}
	}
	g.selection = column + 12
	if column <= 11 {
		g.squadSlot = column
	} else if column == 13 {
		g.squadSlot = 12
	}
}

func (g *Game) activateSquadCursor() {
	if g.squadCursor < 0 {
		g.finishSquad()
		return
	}
	switch g.selection {
	case 30:
		g.toggleSquadController(false)
	case 31:
		g.toggleSquadController(true)
	default:
		g.assignSquadPlayer(g.squadCursor, g.squadSlot)
	}
}

// A controller is first unbound, then assigned to a selected outfield shirt.
// The original menu rejects unassigned records, keepers and both reserves.
func (g *Game) toggleSquadController(partner bool) {
	team := g.squadTeam
	role := uint8(1)
	if !partner && g.competitionSquadController == 1 && team == 1 && g.config.Human[0] {
		role = 3
	}
	if partner {
		if !g.config.Cooperative[team] {
			return
		}
		role = 2
	}
	controller := uint8(g.squadControllerNumber(partner))
	bit := uint8(1 << (controller - 1))
	pool := g.squadPool()
	// 74BC scans original record order, independently of the native view sort.
	ordered := append([]engine.SquadPlayer(nil), pool...)
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].RecordIndex < ordered[j].RecordIndex })
	for _, player := range ordered {
		if g.squadPoolController(player) == controller {
			if slot := g.selectedSquadRecordSlot(team, player.RecordIndex); slot >= 0 {
				g.config.SquadBindings[team][slot] = 0
			} else {
				g.config.UnassignedSquadControllers[team][player.RecordIndex] = 0
			}
			g.squadControllerBits &^= bit
			g.syncSquadBindingSlots()
			g.notice = ""
			g.saveSettings()
			return
		}
	}
	if g.squadCursor < 0 || g.squadCursor >= len(pool) {
		return
	}
	target := pool[g.squadCursor]
	slot := g.selectedSquadRecordSlot(team, target.RecordIndex)
	if slot >= 0 {
		g.config.SquadBindings[team][slot] = 0
	} else {
		g.config.UnassignedSquadControllers[team][target.RecordIndex] = 0
	}
	g.syncSquadBindingSlots()
	if slot < 1 || slot > 10 {
		g.saveSettings()
		return
	}
	g.config.SquadBindings[team][slot] = role
	g.squadControllerBits |= bit
	g.syncSquadBindingSlots()
	g.notice = ""
	g.saveSettings()
}

func (g *Game) squadControllerNumber(partner bool) int {
	if !partner {
		if g.competitionSquadController > 0 && g.squadTeam == 1 {
			return g.competitionSquadController
		}
		if g.squadTeam == 1 && !g.config.Human[0] {
			return 1
		}
		return g.squadTeam + 1
	}
	if g.practiceMode >= 0 && g.practicePlayers == 2 && g.squadTeam == 0 {
		return 2
	}
	return g.squadTeam + 3
}

func (g *Game) squadMenuLabels() map[int]string {
	labels := map[int]string{
		32: "SELECT TEAM " + g.config.TeamNames[g.squadTeam],
		30: fmt.Sprintf("JOYSTICK %d", g.squadControllerNumber(false)),
		31: "SKILL LEVEL",
	}
	if g.config.Cooperative[g.squadTeam] {
		labels[31] = fmt.Sprintf("JOYSTICK %d", g.squadControllerNumber(true))
	}
	return labels
}

func (g *Game) squadBindingLabel(slot int) string {
	if slot < 0 || slot >= 13 {
		return ""
	}
	if g.config.SquadBindings[g.squadTeam][slot] == 1 {
		return fmt.Sprintf("J%d", g.squadControllerNumber(false))
	}
	if g.config.SquadBindings[g.squadTeam][slot] == 3 {
		return "J1"
	}
	if g.config.Cooperative[g.squadTeam] && g.config.SquadBindings[g.squadTeam][slot] == 2 {
		return fmt.Sprintf("J%d", g.squadControllerNumber(true))
	}
	return ""
}
