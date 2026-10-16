package engine

import "math"

const originalFixedScale = 65536.0

// OriginalBallState preserves the signed 16.16 values used by the Amiga's
// central-field integrator. Velocities are displacement per simulation step,
// whereas Ball exposes pitch units per second. Curve is the corner's Y impulse.
type OriginalBallState struct {
	X, Y, Height               int32
	VelocityX, VelocityY, Lift int32
	Curve                      int32
}

// OriginalBallStep translates 0xa474..0xa51a and 0xa548 into integer operations.
// It covers wind, air resistance, gravity and surface impacts. Goal-frame,
// net, boundary and player interactions are separate routines in the original.
// The return value identifies an impact loud enough to play a bounce sample.
func OriginalBallStep(state OriginalBallState, pitch Pitch, windX, windY int32) (OriginalBallState, bool) {
	if state.Curve != 0 {
		state.VelocityY += state.Curve
		state.Curve -= state.Curve >> 4
		state.Curve--
	}
	state.Lift -= 0xd84
	if state.Height>>16 <= 4 {
		windX, windY = 0, 0
	}
	state.VelocityX += (windX - state.VelocityX) >> 7
	state.VelocityY += (windY - state.VelocityY) >> 7
	state.Lift += (-state.Lift) >> 7
	state.Height += state.Lift
	audible := false
	if state.Height < 0 {
		state.Lift = -state.Lift
		audible = state.Lift >= 0x8000
		// The original restores the previous positive height on an impact;
		// setting height to zero would change the next integer step.
		state.Height += state.Lift
		vertical, planar := originalSurfaceShifts(pitch)
		state.Lift -= state.Lift >> vertical
		state.VelocityX -= state.VelocityX >> planar
		state.VelocityY -= state.VelocityY >> planar
	}
	state.X += state.VelocityX
	state.Y += state.VelocityY
	return state, audible
}

// OriginalBounceSample selects the one-based source sample at 0xa4d6 before
// surface restitution. It uses incoming lift after gravity and air resistance.
func OriginalBounceSample(before OriginalBallState) int {
	lift := before.Lift - 0xd84
	lift += (-lift) >> 7
	if before.Height+lift >= 0 {
		return 0
	}
	incoming := -lift
	if incoming < 0x8000 {
		return 0
	}
	return clampInt(int((incoming*2)>>16), 0, 3) + 1
}

// The source order at 0x18436 is normal, wet, soggy, artificial.
func originalSurfaceShifts(pitch Pitch) (uint, uint) {
	switch pitch {
	case Wet:
		return 2, 6
	case Soggy:
		return 1, 2
	case Artificial:
		return 4, 6
	default:
		return 2, 5
	}
}

func originalBallState(ball Ball, curve int32) OriginalBallState {
	return OriginalBallState{X: int32(math.Round(ball.Pos.X * originalFixedScale)),
		Y: int32(math.Round(ball.Pos.Y * originalFixedScale)), Height: int32(math.Round(ball.Height * originalFixedScale)),
		VelocityX: int32(math.Round(ball.Vel.X * originalFixedScale / TPS)),
		VelocityY: int32(math.Round(ball.Vel.Y * originalFixedScale / TPS)),
		Lift:      int32(math.Round(ball.VerticalSpeed * originalFixedScale / TPS)), Curve: curve}
}

func (ball *Ball) applyOriginalState(state OriginalBallState) {
	ball.Pos = Vec2{float64(state.X) / originalFixedScale, float64(state.Y) / originalFixedScale}
	ball.Height = float64(state.Height) / originalFixedScale
	ball.Vel = Vec2{float64(state.VelocityX) * TPS / originalFixedScale, float64(state.VelocityY) * TPS / originalFixedScale}
	ball.VerticalSpeed = float64(state.Lift) * TPS / originalFixedScale
}

// OriginalDirection returns the eight movement vectors at 0xab3a. Their
// diagonal magnitude is 707/1000, and is intentionally not renormalized.
func OriginalDirection(direction int) Vec2 {
	return [...]Vec2{{0, -1}, {.707, -.707}, {1, 0}, {.707, .707}, {0, 1}, {-.707, .707}, {-1, 0}, {-.707, -.707}}[direction&7]
}

// OriginalDirectionIndex reproduces the 2:1 octant thresholds at 0xb8c8.
func OriginalDirectionIndex(delta Vec2) int {
	x, y := math.Abs(delta.X), math.Abs(delta.Y)
	if 2*x < y {
		if delta.Y > 0 {
			return 4
		}
		return 0
	}
	if 2*y < x {
		if delta.X < 0 {
			return 6
		}
		return 2
	}
	if delta.X < 0 {
		if delta.Y > 0 {
			return 5
		}
		return 7
	}
	if delta.Y > 0 {
		return 3
	}
	return 1
}

// OriginalPlayerStep is the word pace multiplied by the direction table before
// adding to a player's 16.16 coordinates at 0xab10..0xab1a. Human runners receive
// twenty extra pace units at 0x107aa..0x107b0.
func OriginalPlayerStep(rawPace byte, direction int, human bool) Vec2 {
	pace := 180 + int(rawPace>>2)
	if human {
		pace += 20
	}
	return OriginalDirection(direction).Mul(float64(pace) * 1000 / originalFixedScale)
}

// OriginalCornerState translates the nine delivery choices at 0xc67c. Charge
// is the number of held-button steps; the original applies min(charge,20)+4 to
// the integer words of its three launch velocities and keeps their fractions.
func OriginalCornerState(position Vec2, choice, charge int) OriginalBallState {
	choice = clampInt(choice, 0, 8)
	if position.X >= PitchWidth/2 && choice != 0 && choice != 1 && choice != 5 {
		choice = 10 - choice
	}
	vertical := [...]int32{100000, 50000, 50000, 100000, 150000, 150000, 150000, 100000, 50000}
	curve := [...]int32{0, 0, -4500, -4500, -4500, 0, 4500, 4500, 4500}
	state := OriginalBallState{X: int32(math.Round(position.X * originalFixedScale)),
		Y: int32(math.Round(position.Y * originalFixedScale)), VelocityX: 500000, VelocityY: 65536,
		Lift: vertical[choice], Curve: curve[choice]}
	if position.X >= PitchWidth/2 {
		state.VelocityX = -state.VelocityX
	}
	if position.Y >= PitchHeight/2 {
		state.VelocityY = -state.VelocityY
	}
	power := clampInt(charge, 0, 20) + 4
	scaleWord := func(value int32) int32 {
		integer := int16(int16(value>>16)*int16(power)) >> 4
		return int32(integer)<<16 | int32(uint16(value))
	}
	state.VelocityX = scaleWord(state.VelocityX)
	state.VelocityY = scaleWord(state.VelocityY)
	state.Lift = scaleWord(state.Lift)
	return state
}

// OriginalShotState translates 0xc474..0xc584 for a supplied sequence of ten
// random words. Skill is the initialized shooting word in 0..100. The source
// scatters a fixed 500-unit planar shot; shooting ability controls its error.
func OriginalShotState(direction, skill int, words [10]uint16) OriginalBallState {
	direction, skill = direction&7, clampInt(skill, 0, 100)
	base, perpendicular := OriginalDirection(direction), OriginalDirection(direction+2)
	lateral := 0
	for i, word := range words[:4] {
		value := int(word) % (110 - skill)
		if i%2 == 0 {
			lateral += value
		} else {
			lateral -= value
		}
	}
	lateral *= 4
	lift := 108
	for i, word := range words[4:] {
		value := int(word) % (101 - skill)
		if i%2 == 0 {
			lift += value
		} else {
			lift -= value
		}
	}
	return OriginalBallState{VelocityX: int32(math.Round((500*base.X + float64(lateral)*perpendicular.X) * 1000)),
		VelocityY: int32(math.Round((500*base.Y + float64(lateral)*perpendicular.Y) * 1000)), Lift: int32(lift * 1000)}
}

func (m *Match) originalShot(direction Vec2, skill float64) OriginalBallState {
	var words [10]uint16
	for i := range words {
		words[i] = uint16(m.random() * 65536)
	}
	return OriginalShotState(OriginalDirectionIndex(direction), int(math.Round(skill*100)), words)
}

// OriginalDribbleState is the independent nudge at 0xc164. The pace argument
// is the initialized pace word, without the human's free-running +20 bonus.
func OriginalDribbleState(direction, pace int) OriginalBallState {
	power := pace + (pace >> 3) + 30
	delta := OriginalDirection(direction).Mul(float64(power) * 1000)
	return OriginalBallState{VelocityX: int32(math.Round(delta.X)), VelocityY: int32(math.Round(delta.Y)), Lift: 40000}
}

// OriginalAftertouchImpulse translates 0xc42a. The saved ball heading chooses
// the perpendicular direction; negative lateral input reverses that direction.
func OriginalAftertouchImpulse(state OriginalBallState, direction, lateral, vertical int) OriginalBallState {
	if lateral >= 0 {
		direction += 2
	} else {
		direction -= 2
		lateral = -lateral
	}
	perpendicular := OriginalDirection(direction).Mul(1000)
	state.VelocityX += (int32(perpendicular.X) * int32(lateral)) >> 2
	state.VelocityY += (int32(perpendicular.Y) * int32(lateral)) >> 2
	state.Lift -= int32(vertical * 200)
	return state
}

// OriginalShotAftertouchImpulse translates the regular shot table at 0x19532
// and routine 0x10570. Its input is relative to the heading saved at launch.
// A forward press lowers lift, a backward press raises it, and side presses
// add a perpendicular impulse. This differs from free-kick aftertouch.
func OriginalShotAftertouchImpulse(state OriginalBallState, launchDirection, inputDirection int) OriginalBallState {
	relative := (inputDirection - launchDirection) & 7
	turn := [...]int{-1, 2, 2, 2, -1, 6, 6, 6}[relative]
	vertical := [...]int{-600, -600, 0, 1000, 1000, 1000, 0, -600}[relative]
	if turn >= 0 {
		impulse := OriginalDirection(launchDirection + turn).Mul(10000)
		state.VelocityX += int32(math.Round(impulse.X))
		state.VelocityY += int32(math.Round(impulse.Y))
	}
	state.Lift += int32(vertical * 10)
	return state
}

// OriginalHeaderState translates 0xc208: the moving step plus a 100-unit
// impulse in the facing direction, with raw upward displacement 60000.
func OriginalHeaderState(direction, step int) OriginalBallState {
	return OriginalDirectedHeaderState(direction, direction, step)
}

// OriginalDirectedHeaderState retains C1EA/C20C's distinct aiming and movement
// directions. The step follows the aimed direction; the extra100 follows the
// unchanged jumper heading, so an NPC can head backward without turning.
func OriginalDirectedHeaderState(aim, movement, step int) OriginalBallState {
	velocity := OriginalDirection(aim).Mul(float64(step) * 1000).Add(OriginalDirection(movement).Mul(100000))
	return OriginalBallState{VelocityX: int32(math.Round(velocity.X)), VelocityY: int32(math.Round(velocity.Y)), Lift: 60000}
}

// OriginalJumpHeight is the source rendering offset at 0x184c6. Each of the
// twenty-two animation phases lasts two updates; negative offsets lift the body.
func OriginalJumpHeight(ticksRemaining int) float64 {
	offsets := [...]int{4, 4, 2, 0, -2, -2, -4, -4, -6, -6, -6, -6, -4, -4, -2, -2, 0, 2, 4, 4, 4, 2}
	phase := clampInt((44-ticksRemaining)/2, 0, 21)
	return float64(-offsets[phase])
}
