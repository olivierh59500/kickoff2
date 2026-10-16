package autoplay

import (
	"math"

	"github.com/olivierh59500/kickoff2/internal/engine"
)

// interceptionPoint receives positions already projected through perception
// delay. Low rolling balls need only a short additional pursuit lead. Air balls
// use their estimated descending low-contact point and ordinary air resistance.
// Motion comes from delayed visual differences, never source velocity words.
func (c *Controller) interceptionPoint(o Observation) engine.Vec2 {
	lead := 0.10
	if o.Height > 8 && c.ownTouchFrames == 0 {
		vertical := c.verticalVelocity - 132*float64(c.options.ReactionFrames)/engine.TPS
		discriminant := vertical*vertical + 2*132*(o.Height-8)
		if discriminant >= 0 {
			lead = math.Max(0.04, math.Min(2, (vertical+math.Sqrt(discriminant))/132))
		}
	}
	travel := lead
	if o.Height > 8 {
		travel = (1 - math.Exp(-0.39*lead)) / 0.39
	}
	return o.Ball.Add(c.velocity.Mul(travel))
}

// avoidSampledAerialReversal avoids reversing a direction that may still be retained
// during ordinary pursuit. A brief issued turn is not proof that the game
// sampled it. Keep recent directions until a new gesture is sustained or the
// old direction expires. Deliberate Fire and follow-through retain their input.
func (c *Controller) avoidSampledAerialReversal(in engine.Input, o Observation, commandFrame int64) engine.Input {
	if in.Fire || o.Height <= 12 || c.aftertouchFrames != 0 || c.shotHoldFrames != 0 || c.visibleCurveFrames != 0 || c.macroPhase != 0 {
		return in
	}
	var blocked [8]bool
	last, held := -1, 0
	for age := int64(1); age <= 5; age++ {
		for index := len(c.commands) - 1; index >= 0; index-- {
			command := c.commands[index]
			if command.frame != commandFrame-age {
				continue
			}
			direction := Direction(command.move)
			if direction == last {
				held++
			} else {
				last, held = direction, 1
			}
			if direction >= 0 {
				blocked[(direction+4)%8] = true
			}
			break
		}
		if last >= 0 && held >= c.options.PursuitGestureFrames {
			break
		}
	}
	desired := Direction(in.Move)
	if desired < 0 || !blocked[desired] {
		return in
	}
	best, score := engine.Vec2{}, -math.MaxFloat64
	for direction := range blocked {
		if blocked[direction] {
			continue
		}
		move := Movement(direction)
		alignment := move.Dot(in.Move)
		if alignment > score {
			best, score = move, alignment
		}
	}
	in.Move = best
	return in
}

// steadyPursuitInput gives ordinary direction changes a short motor commitment.
// Single-sample turns are unreliable in the original input path. Fire, release,
// player changes and learned shooting gestures keep their immediate response.
func (c *Controller) steadyPursuitInput(in engine.Input, o Observation, frame int64) engine.Input {
	if in.Fire || c.lastFire || o.Phase != engine.Playing || o.Controlled != c.lastCommandPlayer || Direction(in.Move) < 0 || Direction(c.lastMove) < 0 {
		return in
	}
	switch c.decision {
	case "", "intercept", "acquire-stationary-ball", "committed-approach", "await-clearance":
	default:
		return in
	}
	held := 0
	for index := len(c.commands) - 1; index >= 0 && held < c.options.PursuitGestureFrames; index-- {
		command := c.commands[index]
		if command.frame != frame-1-int64(held) || command.player != o.Controlled || Direction(command.move) != Direction(c.lastMove) {
			break
		}
		held++
	}
	if held > 0 && held < c.options.PursuitGestureFrames {
		in.Move = c.lastMove
	}
	return in
}

// avoidAerialReversal uses our own issued commands to avoid the original
// opposite-direction bicycle gesture during ordinary pursuit. Its retained
// direction expires after five centered updates. Deliberate Fire and shot
// follow-through gestures retain their requested input.
func (c *Controller) avoidAerialReversal(in engine.Input, o Observation, commandFrame int64) engine.Input {
	if in.Fire || o.Height <= 12 || c.aftertouchFrames != 0 || c.shotHoldFrames != 0 || c.visibleCurveFrames != 0 || c.macroPhase != 0 {
		return in
	}
	retained := -1
	for age := int64(1); age <= 5 && retained < 0; age++ {
		for _, command := range c.commands {
			if command.frame == commandFrame-age && command.move.Len() > 0.1 {
				retained = Direction(command.move)
				break
			}
		}
	}
	if retained >= 0 && Direction(in.Move) == (retained+4)%8 {
		left, right := Movement((retained+2)%8), Movement((retained+6)%8)
		if right.Y*float64(o.Attack) > left.Y*float64(o.Attack) {
			left = right
		}
		in.Move = left
	}
	return in
}
