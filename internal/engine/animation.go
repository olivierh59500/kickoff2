package engine

import "math"

// OriginalOfficialVisual exposes the referee pose used by the source renderer.
// Practice does not draw officials; replay hosts preserve this value per frame.
func (m *Match) OriginalOfficialVisual() (OriginalOfficialState, bool) {
	p := &m.originalOfficial
	state := OriginalOfficialState{X: int32(math.Round(p.Pos.X * originalFixedScale)), Y: int32(math.Round(p.Pos.Y * originalFixedScale)),
		GestureStarted: m.originalOfficialGestureStarted,
		Direction:      OriginalDirectionIndex(p.Facing), Step: p.originalStep, Pace: originalPace(p), State: p.originalState,
		DirectionHistory: p.originalDirectionHistory, AnimationPhase: p.AnimationPhase}
	return state, m.originalHasOfficial && m.originalOfficialsEnabled && !m.Config.Practice
}

// OriginalOfficialSprite selects B5E4's runner or B680/B6C6/B70C's card poses.
// The three gesture lengths share the word table at186B8. Card banks800/720
// are normalized to the decoded referee sheet's initial global bank720.
func OriginalOfficialSprite(official OriginalOfficialState) int {
	if official.State < 4 || official.State > 6 || !official.GestureStarted {
		return (official.Direction&7)*8 + OriginalRunningFrame(official.AnimationPhase, false)
	}
	length, base := 36+4*(official.State-4), 0
	if official.State >= 5 {
		base = 80
	}
	phase := length - 1
	if int16(official.AnimationPhase) >= 0 {
		phase = clampInt((int(official.AnimationPhase)-128)>>8, 0, length-1)
	}
	pose := phase / 4
	if phase < 32 {
		pose %= 4
	} else {
		pose -= 4
	}
	return base + pose
}

// advanceOriginalAnimation reproduces the distance-driven runner accumulator at
// 0xaab6 and 0xaefc. The source's clear-on-zero is overwritten by its cached word,
// so stationary runners preserve phase instead of sharing a global animation clock.
func advanceOriginalAnimation(player *Player) {
	if player.Keeper {
		switch player.originalState {
		case 1, 2, 3, 4, 5, 6, 18, 20, 21, 22, 26, 27, 33, 43, 51, 53, 54, 55, 60, 61, 62:
		default:
			return
		}
	} else {
		if !player.originalVisible {
			return
		}
		switch player.originalState {
		case 1, 2, 3, 4, 5, 6, 7, 8, 9, 11, 16, 17, 21, 22, 23, 24, 25, 26, 27, 28, 30, 31, 33, 36, 41, 42, 43, 54, 55, 56, 57, 58, 59, 60, 61, 62:
		default:
			return
		}
	}
	phase := player.AnimationPhase + uint16(player.originalStep*2)
	if phase>>8 >= 38 {
		phase -= 0x2600
		if player.Keeper {
			player.originalKeeperCarryCycles++
		}
	}
	player.AnimationPhase = phase
}

// OriginalRunningFrame selects the original runner/keeper table entry using the
// high byte of the per-player fixed accumulator. No renderer clock is involved.
func OriginalRunningFrame(phase uint16, keeper bool) int {
	index := int(phase>>8) % 38
	if keeper {
		return originalKeeperFrames[index]
	}
	return originalRunnerFrames[index]
}

var originalRunnerFrames = [38]int{0, 0, 3, 3, 3, 3, 3, 3, 2, 2, 2, 2, 1, 1, 1, 2, 2, 3, 3, 4, 4, 7, 7, 7, 7, 7, 7, 6, 6, 6, 6, 5, 5, 5, 6, 6, 7, 7}
var originalKeeperFrames = [38]int{0, 0, 3, 3, 3, 3, 3, 3, 2, 2, 2, 2, 1, 1, 1, 2, 2, 3, 3, 0, 0, 6, 6, 6, 6, 6, 6, 5, 5, 5, 5, 4, 4, 4, 5, 5, 6, 6}

// OriginalJumpFrame selects the four-pose airborne runner animation at0xac92.
// A negative return means the player is not in a jump animation.
func (player Player) OriginalJumpFrame() int {
	phase := 21
	if player.originalState == 13 {
		if int16(player.AnimationPhase) >= 0 {
			phase = clampInt((int(player.AnimationPhase)-128)>>8, 0, 21)
		}
	} else {
		// Older portable replays contain the visual countdown without the
		// source action/phase fields; retain their original interpretation.
		if player.jumpTicks <= 0 {
			return -1
		}
		phase = clampInt((44-player.jumpTicks)/2, 0, 21)
	}
	table := [22]int{0, 0, 1, 1, 1, 1, 2, 2, 2, 2, 2, 3, 3, 3, 3, 2, 2, 1, 1, 0, 0, 1}
	return table[phase]
}

// VisualJumpTicks returns the animation countdown used by replay frames.
func (player Player) VisualJumpTicks() int { return player.jumpTicks }

// RestoreVisualJumpTicks restores an already recorded animation countdown. It
// is used only on detached replay footballers, never the live match or its RNG.
func (player *Player) RestoreVisualJumpTicks(ticks int) { player.jumpTicks = clampInt(ticks, 0, 44) }

// VisualOriginalAction returns only the source action fields needed for drawing.
func (player Player) VisualOriginalAction() (state, substate int) {
	return player.originalState, player.originalSubstate
}

// RestoreVisualOriginalAction restores action fields on a detached replay actor.
// It does not resume that action or advance the live match simulation.
func (player *Player) RestoreVisualOriginalAction(state, substate int) {
	player.originalState, player.originalSubstate = state, substate
}

// OriginalSpecialSprite translates the non-running frame indices and offsets
// at AD26, ADC8, AE62, AFA0, B044, B0DA and B170. Returned indices address the
// individual decoded kit/keeper sheet, rather than the original global bank.
func (player Player) OriginalSpecialSprite() (index, offsetY int, present bool) {
	direction := OriginalDirectionIndex(player.Facing)
	phase := int(player.AnimationPhase >> 8)
	if player.Keeper {
		if player.originalSubstate == 45 || player.originalSubstate == 46 || player.originalSubstate == 48 || player.originalState == 47 || player.originalState == 49 || player.originalState == 50 {
			phase = 19
			if int16(player.AnimationPhase) >= 0 {
				phase = clampInt((int(player.AnimationPhase)-170)>>8, 0, 19)
			}
			base := 184
			if player.originalState == 47 || player.originalState == 49 || player.originalState == 50 {
				base = 200
			}
			if direction < 4 {
				base += 4
			}
			if (direction-2)&7 < 4 {
				base += 8
			}
			offset := originalKeeperLowDiveY[phase]
			if player.originalSubstate == 48 || player.originalState == 49 {
				offset = originalKeeperHighDiveY[phase]
			}
			return base + originalKeeperDiveFrames[phase], offset - 3, true
		}
		if player.originalState == 39 || player.originalSubstate == 37 || player.originalSubstate == 38 {
			phase = 14
			if int16(player.AnimationPhase) >= 0 {
				phase = clampInt((int(player.AnimationPhase)-256)>>8, 0, 14)
			}
			switch {
			case player.originalState == 39:
				return 136 + direction*3 + originalKeeperLowerFrames[phase], originalKeeperLowerY[phase] - 3, true
			case player.originalSubstate == 37:
				return 112 + direction*3 + originalKeeperRiseFrames[phase], originalKeeperRiseY[phase] - 3, true
			default:
				return 112 + direction*3 + originalKeeperLowerFrames[phase], originalKeeperLowerY[phase] - 3, true
			}
		}
		if player.originalState == 19 {
			if int16(player.AnimationPhase) < 0 {
				phase = 2
			}
			return 160 + direction*3 + clampInt(phase, 0, 2), 0, true
		}
		if player.originalState == 20 || player.originalState == 44 {
			return 56 + direction*7 + OriginalRunningFrame(player.AnimationPhase, true), 0, true
		}
		return 0, 0, false
	}
	switch player.originalState {
	case 10, 29:
		// E5DA keeps the throw pose until another actor touches the ball.
		return 64 + direction*3 + clampInt(phase, 0, 2), 0, true
	case 14:
		return 148 + direction*3 + clampInt(phase, 0, 2), 0, true
	case 15:
		return 172 + direction*3 + clampInt(phase, 0, 2), 0, true
	case 35:
		// B4C0 advances by128 before exporting the phase. Its two word
		// tables are at1849A/184C6, with four poses for each facing pair.
		phase = 21
		if int16(player.AnimationPhase) >= 0 {
			phase = clampInt((int(player.AnimationPhase)-128)>>8, 0, 21)
		}
		// B4C0 adds global252 to the team bank. That belongs to the
		// global220..319 segment, which begins at local kit cell100.
		return 132 + (direction>>1)*4 + originalCelebrationFrames[phase], originalCelebrationY[phase], true
	case 63:
		phase = 15
		if int16(player.AnimationPhase) >= 0 {
			phase = clampInt((int(player.AnimationPhase)-128)>>8, 0, 15)
		}
		return 200 + ((direction+4)&7)*3 + originalScissorFrames[phase], originalScissorY[phase], true
	}
	return 0, 0, false
}

var originalScissorFrames = [16]int{0, 0, 0, 0, 1, 1, 1, 1, 2, 2, 2, 2, 2, 2, 2, 2}
var originalCelebrationFrames = [22]int{0, 0, 1, 1, 1, 1, 2, 2, 2, 2, 2, 3, 3, 3, 3, 2, 2, 1, 1, 0, 0, 1}
var originalCelebrationY = [22]int{4, 4, 2, 0, -2, -2, -4, -4, -6, -6, -6, -6, -4, -4, -2, -2, 0, 2, 4, 4, 4, 2}
var originalScissorY = [16]int{4, 4, 2, 0, -2, -2, -4, -4, -2, -2, 0, 2, 4, 4, 4, 4}
var originalKeeperRiseFrames = [15]int{0, 0, 0, 1, 1, 1, 1, 1, 2, 2, 2, 2, 2, 2, 2}
var originalKeeperRiseY = [15]int{0, -2, -4, -4, -6, -6, -8, -8, -8, -10, -10, -10, -12, -12, -12}
var originalKeeperLowerFrames = [15]int{2, 2, 2, 2, 2, 2, 2, 1, 1, 1, 1, 1, 0, 0, 0}
var originalKeeperLowerY = [15]int{-12, -12, -12, -10, -10, -10, -8, -8, -8, -6, -6, -4, -4, -2, 0}
var originalKeeperDiveFrames = [20]int{0, 0, 0, 0, 1, 1, 1, 1, 1, 1, 2, 2, 2, 2, 2, 2, 2, 2, 2, 3}
var originalKeeperHighDiveY = [20]int{0, -1, -3, -5, -7, -8, -9, -10, -11, -12, -11, -10, -9, -8, -7, -5, -3, -1, 2, 4}
var originalKeeperLowDiveY = [20]int{0, 0, -1, -2, -2, -3, -3, -3, -4, -4, -4, -3, -3, -3, -2, -2, -1, 0, 2, 4}

// OriginalCameraOffset returns the top-left field coordinate tracked by the
// original 0x11492 scrolling kernel. Presentation adds its PAL raster margin.
func (match *Match) OriginalCameraOffset() Vec2 {
	return Vec2{float64(match.originalDisplayCameraX) / originalFixedScale, float64(match.originalDisplayCameraY) / originalFixedScale}
}

// OriginalBallVisible follows the source drawing flag. Held keeper/thrower
// sprites contain their own ball, so the independent ball sprite is hidden.
func (match *Match) OriginalBallVisible() bool { return match.originalBallDrawEnabled }

// OriginalBallSpriteLayout translates0xa39a..0xa430. Ball size depends on
// integer elevation; high balls use a separate projected shadow to their right.
func OriginalBallSpriteLayout(height float64) (frame, elevation int, shadow bool) {
	elevation = max(0, int(height)) >> 1
	if elevation < 4 {
		return elevation, elevation, false
	}
	return 4 + min(3, elevation>>4), elevation, true
}
