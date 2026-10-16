package autoplay

import (
	"math"

	"github.com/olivierh59500/kickoff2/internal/engine"
)

// visibleFireAttempt remembers an issued gesture, not an engine shot or owner.
// Its ball position and speed come from the same delayed visual observations
// used for ordinary interception. Command time is known without visual delay.
type visibleFireAttempt struct {
	frame  int64
	player int
	ball   engine.Vec2
	speed  float64
}

func (c *Controller) rememberFireAttempt(observed, forecast Observation, input engine.Input, commandFrame int64) {
	if !input.Fire || c.lastFire || observed.Phase != engine.Playing || observed.Controlled < 1 || observed.Controlled > 10 {
		return
	}
	player := forecast.Players[forecast.Team][forecast.Controlled]
	if input.Move.Y*float64(observed.Attack) < 0.5 || forecast.Ball.Sub(player.Pos).Len() > 40 {
		return
	}
	c.fireAttempt = &visibleFireAttempt{commandFrame, observed.Controlled, observed.Ball, c.velocity.Len()}
}

// observeFiredBall recognizes visible acceleration after our own recent Fire.
// In the original v12 near-goal sequence a perceived header actually produced
// an ordinary shot. Waiting for a private action/ownership word would violate
// the observation boundary; following only the old decision missed the flight.
func (c *Controller) observeFiredBall(o Observation, motionReset bool) bool {
	if motionReset || o.Phase != engine.Playing {
		c.fireAttempt, c.visibleCurveFrames = nil, 0
		return false
	}
	if c.visibleCurveFrames > 0 && o.Controlled != c.visibleCurvePlayer {
		c.visibleCurveFrames = 0
	}
	attempt := c.fireAttempt
	if attempt == nil {
		return false
	}
	age := o.Frame - attempt.frame
	if age > 12 || o.Controlled != attempt.player {
		c.fireAttempt = nil
		return false
	}
	if age < 0 || c.velocity.Len() < 300 || c.velocity.Len() < attempt.speed+60 || c.velocity.Y*float64(o.Attack) < 250 || math.Abs(c.velocity.X) > math.Abs(c.velocity.Y)*0.35 {
		return false
	}
	goalY := 0.0
	if o.Attack > 0 {
		goalY = engine.PitchHeight
	}
	distance := math.Abs(goalY - o.Ball.Y)
	player := o.Players[o.Team][attempt.player]
	if distance < 32 || distance > c.options.ShotRange+40 || o.Height > 34 || o.Ball.Sub(attempt.ball).Len() < 10 || o.Ball.Sub(player.Pos).Len() > 70 || math.Abs(o.Ball.X-engine.PitchWidth/2) > engine.GoalWidth/2+16 {
		return false
	}
	keeper := o.Players[1-o.Team][0]
	side := 1.0
	if keeper.Pos.X >= engine.PitchWidth/2 {
		side = -1
	}
	c.shotAimX = engine.PitchWidth/2 + side*(engine.GoalWidth/2-16)
	remaining := (goalY - o.Ball.Y) / c.velocity.Y
	crossing := o.Ball.X + c.velocity.X*remaining
	c.visibleCurveMove = joystick(engine.Vec2{X: math.Copysign(1, c.shotAimX-crossing), Y: float64(o.Attack)})
	c.visibleCurveFrames, c.visibleCurvePlayer = 5, attempt.player
	c.buttonFrames, c.trapFrames, c.shotHoldFrames = 0, 0, 0
	c.macroPhase, c.macroFrames, c.pendingShotFrames, c.aftertouchFrames = 0, 0, 0, 0
	c.fireAttempt = nil
	return true
}

func (c *Controller) visibleShotFollowThrough(o Observation, goal engine.Vec2) engine.Input {
	c.visibleCurveFrames--
	direction := c.visibleCurveMove
	c.decision = "visible-shot-aftertouch"
	remaining := (goal.Y - o.Ball.Y) / c.velocity.Y
	if remaining <= 0 || remaining > 2 {
		c.visibleCurveFrames = 0
		return engine.Input{}
	}
	crossing := o.Ball.X + c.velocity.X*remaining
	left, right := goal.X-engine.GoalWidth/2+12, goal.X+engine.GoalWidth/2-12
	if direction.X < 0 && (crossing <= left || crossing <= c.shotAimX) || direction.X > 0 && (crossing >= right || crossing >= c.shotAimX) {
		// Stop lateral input when delayed evidence already threatens a post.
		// A short forward follow-through still releases Fire and lowers the ball.
		direction = engine.Vec2{Y: float64(o.Attack)}
		c.decision = "visible-shot-follow-through"
	}
	return engine.Input{Move: direction}
}
