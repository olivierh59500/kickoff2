package engine

import (
	"encoding/binary"
	"math"

	"github.com/olivierh59500/kickoff2/internal/replaydata"
)

// originalVideoState retains display commands at their source draw points.
// A later keeper touch, controller selection or camera callback must not move
// sprites that have already been drawn during this world update.
type originalVideoState struct {
	frame             replaydata.Frame
	selectorCommands  []replaydata.Command
	selectorOrigin    [2]int16
	selectorVariant   int
	selectorAt        int
	selectorChoice    int
	selectorRecording bool
	ready             bool
	nameTeam          int
	nameActor         int
	scanner           OriginalScannerState
	caption           replaydata.Command
	hasCaption        bool
	cameraX, cameraY  int32
	actorImages       [2][11]originalRetainedSprite
	actorsDrawn       [2][11]bool
	controllerImages  [2][2]originalRetainedSprite
	controllersDrawn  [2]bool
}

// Source EEA0 erases the previous object only when its next draw is reached.
// A blocked actor leaves that object and later objects on the foreground plane.
type originalRetainedSprite struct {
	command replaydata.Command
	visible bool
}

// OriginalPlayerImage returns the last retained drawing for a footballer.
// The copied command exposes visible appearance without action or phase words.
func (m *Match) OriginalPlayerImage(team, actor int) (replaydata.Command, bool) {
	if team < 0 || team >= 2 || actor < 0 || actor >= 11 {
		return replaydata.Command{}, false
	}
	image := m.originalVideo.actorImages[team][actor]
	if !image.visible {
		return replaydata.Command{}, false
	}
	return image.command, true
}

// OriginalScannerPoint stores the source's retained map marker, not a fresh
// projection of every player on every host Draw call.
type OriginalScannerPoint struct {
	X, Y int16
	Big  bool
}

// OriginalScannerState follows the shared14E48 throttle and each actor's20/22
// words. Source division truncates signed integer coordinates toward zero.
type OriginalScannerState struct {
	Countdown, Mode, OriginX, OriginY, Scale int16
	Ball                                     OriginalScannerPoint
	Players                                  [2][11]OriginalScannerPoint
}

func (m *Match) initializeOriginalScanner() {
	s := &m.originalVideo.scanner
	s.Countdown = 0
	s.Mode, s.OriginX, s.OriginY, s.Scale = 1, 24, 28, 16
	s.Ball.X = -1
	for team := range s.Players {
		for actor := range s.Players[team] {
			s.Players[team][actor].X = -1
		}
	}
}

func (m *Match) updateOriginalScannerActor(team, actor int) {
	s := &m.originalVideo.scanner
	if s.Mode == 0 || s.Scale <= 0 {
		return
	}
	s.Countdown--
	if s.Countdown >= 0 {
		return
	}
	s.Countdown = 6
	p := &m.Teams[team].Players[actor]
	if p.originalState == 0 {
		return
	}
	point := &s.Players[team][actor]
	if s.Mode < 0 {
		s.Mode++
		point.X = -1
		return
	}
	point.X = int16(math.Floor(p.Pos.X))/s.Scale + s.OriginX
	point.Y = int16(math.Floor(p.Pos.Y))/s.Scale + s.OriginY
	point.Big = m.originalRecordHasController(team, actor)
}

func (m *Match) beginOriginalVideo(x, y int32) {
	m.originalVideo.actorsDrawn = [2][11]bool{}
	m.originalVideo.controllersDrawn = [2]bool{}
	m.originalVideo.selectorCommands = m.originalVideo.selectorCommands[:0]
	m.originalVideo.selectorRecording = false
	m.originalVideo.cameraX, m.originalVideo.cameraY = m.originalCameraX, m.originalCameraY
	m.originalVideo.frame.FocusX, m.originalVideo.frame.FocusY = int16(x>>16), int16(y>>16)
	m.originalVideo.frame.Commands = m.originalVideo.frame.Commands[:0]
	m.originalVideo.frame.Complete = true
	m.originalVideo.ready = true
}

func (m *Match) originalVideoSprite(position Vec2, code, offsetY int) {
	x := int(math.Floor(position.X)) - int(m.originalCameraX>>16) - 8
	y := int(math.Floor(position.Y)) - int(m.originalCameraY>>16) - 8 + offsetY
	// EEA0/EE56 use unsigned origin clipping before writing a ring command.
	if x < 0 || x >= 335 || y < 0 || y >= 239 {
		return
	}
	m.originalVideo.frame.Commands = append(m.originalVideo.frame.Commands, replaydata.Command{X: int16(x), Y: int16(y), Code: int16(code)})
}

func (m *Match) updateOriginalScannerBall() {
	s := &m.originalVideo.scanner
	if s.Mode > 0 && s.Scale > 0 {
		s.Ball.X = int16(math.Floor(m.Ball.Pos.X))/s.Scale + s.OriginX
		s.Ball.Y = int16(math.Floor(m.Ball.Pos.Y))/s.Scale + s.OriginY
	}
}

func (m *Match) recordOriginalBallVideo() {
	if !m.originalBallDrawEnabled {
		return
	}
	frame, elevation, shadow := OriginalBallSpriteLayout(m.Ball.Height)
	if shadow {
		m.originalVideoSprite(m.Ball.Pos.Add(Vec2{X: float64(elevation), Y: -float64(elevation)}), 208, 0)
	}
	m.originalVideoSprite(m.Ball.Pos, 200+frame, -elevation)
}

func (m *Match) recordOriginalOfficialVideo() {
	official, visible := m.OriginalOfficialVisual()
	if visible {
		m.originalVideoSprite(Vec2{float64(official.X) / 65536, float64(official.Y) / 65536}, 720+OriginalOfficialSprite(official), 0)
	}
}

// OriginalPlayerSprite returns a local sheet cell at a completed actor draw.
// AAB6 uses cell97 when the prior sprite was clipped; that branch does not
// advance the running phase, even if movement has now brought the actor back.
func OriginalPlayerSprite(player Player, previouslyVisible bool) (cell, offsetY int) {
	direction := OriginalDirectionIndex(player.Facing)
	if player.Keeper {
		cell = direction*7 + OriginalRunningFrame(player.AnimationPhase, true)
	} else {
		cell = direction*8 + OriginalRunningFrame(player.AnimationPhase, false)
		if !previouslyVisible {
			cell = 97
		}
		if player.originalState == 12 {
			cell = 88 + direction
		}
		if phase := player.OriginalJumpFrame(); phase >= 0 {
			cell, offsetY = 100+direction*4+phase, -int(player.JumpHeight)
		}
	}
	if special, offset, present := player.OriginalSpecialSprite(); present {
		cell, offsetY = special, offset
	}
	return cell, offsetY
}

func (m *Match) recordOriginalPlayerVideo(team, actor int, previouslyVisible bool, entryState, entrySubstate int) {
	player := &m.Teams[team].Players[actor]
	pose := *player
	if pose.originalState != 0 {
		// Live source actions select the draw routine. The separate jump
		// countdown is retained only for old detached portable replays.
		pose.jumpTicks = 0
	}
	if !player.Keeper && entryState == 13 && player.originalState == 1 && m.Config.Human[team] && (m.Teams[team].primaryControls(actor) || m.originalRecordHasController(team, actor)) {
		// 104EE clears the human action, then still draws through AC92.
		// The following update resumes the ordinary running image.
		pose.originalState = 13
		pose.JumpHeight = OriginalJumpHeight(44)
	}
	if player.Keeper && entryState == 1 && player.originalState == 44 {
		//13E90's fresh catch still returns through the ordinary AEFC pose;
		// the carrying bank begins at the following action44 dispatch.
		pose.originalState = 1
	}
	if player.Keeper && (entrySubstate == 45 && player.originalState == 47 || entrySubstate == 48 && player.originalState == 49) {
		// 143A0/14476 still draw the incoming dive after a successful
		// catch. The caught-ball bank starts on the following dispatch.
		pose.originalState, pose.originalSubstate = 1, entrySubstate
	}
	if !player.Keeper && (player.originalState == 14 && entryState != 14 || player.originalState == 15 && entryState != 14 && entryState != 15) {
		// The contact caller still draws its runner before the newly assigned
		// knockdown/recovery action dispatches on the following update.
		pose.originalState = 1
	}
	cell, offsetY := OriginalPlayerSprite(pose, previouslyVisible)
	code := cell + team*100
	if player.Keeper {
		code = 420 + cell
	} else if cell >= 200 {
		code = 440 + team*40 + cell
	} else if cell >= 100 {
		code = 120 + team*100 + cell
	} else if cell == 97 && !previouslyVisible {
		code = 97 // AB7C bypasses the team-bank addition.
	}
	start := len(m.originalVideo.frame.Commands)
	m.originalVideoSprite(player.Pos, code, offsetY)
	m.originalVideo.actorsDrawn[team][actor] = true
	retained := &m.originalVideo.actorImages[team][actor]
	retained.visible = len(m.originalVideo.frame.Commands) != start
	player.originalVisible = retained.visible
	if retained.visible {
		retained.command = m.originalVideo.frame.Commands[start]
	}
}

func (m *Match) recordOriginalControllerVideo(team int) {
	m.originalVideo.controllersDrawn[team] = true
	m.originalVideo.controllerImages[team] = [2]originalRetainedSprite{}
	if !m.Config.Human[team] {
		return
	}
	partner := m.PartnerPlayer(team)
	if partner >= 0 {
		partner += team * 11
	}
	for marker, actor := range []int{m.Teams[team].ControlledActor(team), partner} {
		if actor < 0 || actor >= 22 {
			continue
		}
		p := &m.Teams[actor/11].Players[actor%11]
		if !p.originalVisible || p.SentOff {
			continue
		}
		command := replaydata.Command{
			X: int16(int(math.Floor(p.Pos.X)) - int(m.originalCameraX>>16) - 8),
			Y: int16(int(math.Floor(p.Pos.Y)) - int(m.originalCameraY>>16) + 8), Code: int16(-1 - marker),
		}
		m.originalVideo.frame.Commands = append(m.originalVideo.frame.Commands, command)
		m.originalVideo.controllerImages[team][marker] = originalRetainedSprite{command, true}
	}
}

func (m *Match) finishOriginalVideo() {
	team, actor := m.Ball.LastTeam, m.Ball.LastPlayer
	if team < 0 || team > 1 || actor < 0 || actor > 10 || team == m.originalVideo.nameTeam && actor == m.originalVideo.nameActor {
		return
	}
	// FA42 first XOR-erases the previous name, then writes the new one.
	for _, identity := range [][2]int{{m.originalVideo.nameTeam, m.originalVideo.nameActor}, {team, actor}} {
		t, index := identity[0], identity[1]
		if t < 0 || t > 1 || index < 0 || index > 10 {
			continue
		}
		command := m.originalNameCommand(t, index)
		m.originalVideo.frame.Commands = append(m.originalVideo.frame.Commands, command)
	}
	m.originalVideo.nameTeam, m.originalVideo.nameActor = team, actor
	m.originalVideo.caption, m.originalVideo.hasCaption = m.originalNameCommand(team, actor), true
}

func (m *Match) originalNameCommand(team, actor int) replaydata.Command {
	p := &m.Teams[team].Players[actor]
	command := replaydata.Command{X: int16(p.Number - 1), Y: int16(-28668 - 2*team), Code: -3}
	copy(command.Name[:16], p.originalNameBytes[:])
	if p.originalNameBytes[0] == 0 {
		for offset, value := range []rune(p.Name) {
			if offset >= 15 {
				break
			}
			command.Name[offset] = byte(value)
		}
	}
	//F998..F9EE copies18 bytes from+36: the16-byte name storage and
	// the following first tactic-target word, including unused name padding.
	binary.BigEndian.PutUint16(command.Name[16:], uint16(int16(p.originalTargets[0].X)))
	return command
}

// OriginalVideoCaption retains the last displayed label while the source's
// last-touch pointer is temporarily cleared during a restart or goal reset.
func (m *Match) OriginalVideoCaption() (replaydata.Command, bool) {
	return m.originalVideo.caption, m.originalVideo.hasCaption
}

// OriginalVideoCamera is the camera published for the drawn playfields. The
// later11492 callback prepares scrolling for the following raster, not the
// sprites already emitted by this update.
func (m *Match) OriginalVideoCamera() (int32, int32) {
	return m.originalVideo.cameraX, m.originalVideo.cameraY
}

// OriginalVideoNextTarget supplies the next source FCA6 marker from current
// native state, without importing an observed camera or future position.
func (m *Match) OriginalVideoNextTarget() (int16, int16) {
	x, y := m.originalCameraFocus()
	return int16(x >> 16), int16(y >> 16)
}

// OriginalVideoPreview supplies a detached initial/legacy set-piece image.
// Normal live updates use the commands retained at their actual draw points.
func (m *Match) OriginalVideoPreview() replaydata.Frame {
	copy := *m
	copy.originalVideo.frame.Commands = nil
	x, y := copy.originalCameraFocus()
	copy.beginOriginalVideo(x, y)
	copy.recordOriginalBallVideo()
	copy.recordOriginalOfficialVideo()
	for team := range copy.Teams {
		for actor := range copy.Teams[team].Players {
			p := &copy.Teams[team].Players[actor]
			if !p.SentOff && p.originalState != 0 {
				copy.recordOriginalPlayerVideo(team, actor, p.originalVisible, p.originalState, p.originalSubstate)
			}
		}
		copy.recordOriginalControllerVideo(team)
	}
	return copy.originalVideo.frame
}

// OriginalVideoFrame copies the last completed update's original command stream.
// Consumers can rewind, render or serialize it without changing live state.
func (m *Match) OriginalVideoFrame() (replaydata.Frame, bool) {
	frame := m.originalVideo.frame
	if len(m.originalVideo.selectorCommands) != 0 {
		at := m.originalVideo.selectorAt
		commands := make([]replaydata.Command, 0, len(frame.Commands)+len(m.originalVideo.selectorCommands))
		commands = append(commands, frame.Commands[:at]...)
		commands = append(commands, m.originalVideo.selectorCommands...)
		frame.Commands = append(commands, frame.Commands[at:]...)
	} else {
		frame.Commands = append([]replaydata.Command(nil), frame.Commands...)
	}
	return frame, m.originalVideo.ready && !m.originalCornerSelecting
}

// OriginalLiveVideoFrame omits the selector's recorded cursor and ball redraw.
// F006 erases them without recording an inverse command; the completed live
// foreground therefore differs from playback of that transient movie frame.
func (m *Match) OriginalLiveVideoFrame() (replaydata.Frame, bool) {
	frame := m.originalVideo.frame
	frame.Commands = append([]replaydata.Command(nil), frame.Commands...)
	if m.originalCornerSelecting {
		// A444 erases the ball/shadow without adding inverse movie commands.
		commands := frame.Commands[:0]
		for _, command := range frame.Commands {
			if command.Code < 200 || command.Code >= 220 {
				commands = append(commands, command)
			}
		}
		frame.Commands = commands
		// These are prior native draw records, not fresh player projections.
		// The suspended taker and later teams have not erased/redrawn them yet.
		for team := range m.Teams {
			for actor, retained := range m.originalVideo.actorImages[team] {
				if !m.originalVideo.actorsDrawn[team][actor] && retained.visible {
					frame.Commands = append(frame.Commands, retained.command)
				}
			}
			if !m.originalVideo.controllersDrawn[team] {
				for _, retained := range m.originalVideo.controllerImages[team] {
					if retained.visible {
						frame.Commands = append(frame.Commands, retained.command)
					}
				}
			}
		}
	}
	return frame, m.originalVideo.ready
}

// OriginalCornerPanelState is detached display data for the blocked selector.
// The source fixes its origin and panel variant once at D69A, before polling.
type OriginalCornerPanelState struct {
	X, Y    int16
	Variant int
	Choice  int
}

// OriginalCornerPanelVisual returns the latched panel while source D69A waits.
func (m *Match) OriginalCornerPanelVisual() (OriginalCornerPanelState, bool) {
	v := &m.originalVideo
	return OriginalCornerPanelState{
		X: v.selectorOrigin[0], Y: v.selectorOrigin[1], Variant: v.selectorVariant, Choice: m.Restart.CornerChoice,
	}, m.originalCornerSelecting
}

func (m *Match) beginOriginalCornerSelectorVideo() {
	m.originalVideo.selectorAt = len(m.originalVideo.frame.Commands)
	m.originalVideo.selectorChoice = -1
	m.originalVideo.selectorRecording = true
	// D69A fixes the panel origin once; later VBlanks do not reposition it.
	x, y := 32, 1360
	m.originalVideo.selectorVariant = 0
	if math.Floor(m.Ball.Pos.X) >= 456 {
		x = 912
		if math.Floor(m.Ball.Pos.Y) <= 696 {
			y, m.originalVideo.selectorVariant = 32, 1
		}
	} else if math.Floor(m.Ball.Pos.Y) >= 696 {
		m.originalVideo.selectorVariant = 1
	} else {
		y = 32
	}
	x = min(223, max(0, x-int(m.originalCameraX>>16)-48)) &^ 15
	y = min(128, max(0, y-int(m.originalCameraY>>16)-48)) &^ 15
	m.originalVideo.selectorOrigin = [2]int16{int16(x), int16(y)}
	m.originalBallDrawEnabled = false
}

// D69A uses the nine signed coordinate pairs at1949A. EE56 records each new
// cursor; F006 removes its pixels without adding a movie command.
func (m *Match) recordOriginalCornerSelectorChoice(choice int) {
	v := &m.originalVideo
	if !v.selectorRecording || choice == v.selectorChoice {
		return
	}
	v.selectorChoice = choice
	offsets := [9][2]int16{{40, 40}, {40, 24}, {56, 24}, {56, 40}, {56, 56}, {40, 56}, {24, 56}, {24, 40}, {24, 24}}
	v.selectorCommands = append(v.selectorCommands, replaydata.Command{X: v.selectorOrigin[0] + offsets[choice][0], Y: v.selectorOrigin[1] + offsets[choice][1], Code: 636})
}

func (m *Match) finishOriginalCornerSelectorVideo() {
	if !m.originalVideo.selectorRecording {
		return
	}
	m.originalVideo.selectorRecording = false
	// D7B2 calls A384 to restore the ball after A444 hid it for the selector.
	// Keep the actual recorder writes, while the live frame retains one ball.
	start := len(m.originalVideo.frame.Commands)
	m.recordOriginalBallVideo()
	m.originalVideo.selectorCommands = append(m.originalVideo.selectorCommands, m.originalVideo.frame.Commands[start:]...)
	m.originalVideo.frame.Commands = m.originalVideo.frame.Commands[:start]
}
