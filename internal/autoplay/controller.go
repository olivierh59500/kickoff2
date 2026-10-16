package autoplay

import (
	"github.com/olivierh59500/kickoff2/internal/engine"
	"math"
)

// Options constrain the controller to plausible reaction and action timings.
type Options struct {
	ReactionFrames      int
	ShotRange           float64
	MinimumButtonFrames int
	// PursuitGestureFrames enables experimental sustained gestures; zero
	// retains the established native baseline's direction decisions.
	PursuitGestureFrames int
}

// DefaultOptions delays perception by six observations, nominally 120 ms at 50 Hz.
func DefaultOptions() Options {
	return Options{ReactionFrames: 6, ShotRange: 210, MinimumButtonFrames: 3}
}

// Controller estimates travel from successive visual observations and produces
// only an eight-direction joystick plus the original single action button.
type Controller struct {
	fireAttempt                                *visibleFireAttempt
	visibleCurveFrames                         int
	visibleCurvePlayer                         int
	visibleCurveMove                           engine.Vec2
	contestWaitFrames                          int
	trapEvidenceFrames                         int
	macroPhase                                 int
	macroFrames                                int
	macroPlayer                                int
	shotHoldFrames                             int
	shotCurveFrames                            int
	shotConfirmed                              bool
	laneStage                                  int
	laneFrames                                 int
	laneCorrections                            int
	laneMove                                   engine.Vec2
	cornerReadyFrames                          int
	cornerFireFrames                           int
	commands                                   []plannedCommand
	runningSpeed                               float64
	wingEstablished                            bool
	attackSide                                 int
	keeperBallFrames                           int
	shotAimX                                   float64
	shotOrigin                                 engine.Vec2
	shotAfterDirection                         engine.Vec2
	decision                                   string
	pendingShotFrames                          int
	diagnostic                                 Diagnostics
	ownTouchFrames                             int
	lastMove                                   engine.Vec2
	lastCommandPlayer                          int
	playerVelocity                             engine.Vec2
	verticalVelocity                           float64
	options                                    Options
	history                                    []Observation
	previous                                   Observation
	velocity                                   engine.Vec2
	initialized                                bool
	buttonFrames, trapFrames, aftertouchFrames int
	lastFire                                   bool
	lastControlled                             int
	lastPhase                                  engine.Phase
	shotDirection                              engine.Vec2
	heldDirection                              engine.Vec2
}

type plannedCommand struct {
	frame  int64
	player int
	move   engine.Vec2
	fire   bool
}

func New(options Options) *Controller {
	options.ReactionFrames = max(0, min(25, options.ReactionFrames))
	options.PursuitGestureFrames = max(0, min(5, options.PursuitGestureFrames))
	if options.ShotRange <= 0 {
		options.ShotRange = 210
	}
	if options.MinimumButtonFrames < 1 {
		options.MinimumButtonFrames = 3
	}
	return &Controller{options: options, lastControlled: -1, lastPhase: engine.FullTime, runningSpeed: 190}
}

// Update consumes one observation. The delay queue prevents instantaneous access
// to the current frame, even in an offline benchmark running faster than real time.
func (c *Controller) Update(current Observation) engine.Input {
	c.history = append(c.history, current)
	if len(c.history) <= c.options.ReactionFrames {
		return engine.Input{}
	}
	o := c.history[0]
	c.history = c.history[1:]
	if c.contestWaitFrames > 0 {
		c.contestWaitFrames--
	}
	motionReset := false
	newAttack := c.initialized && c.previous.Ball.Sub(o.Ball).Len() > 350
	if c.initialized && o.Controlled >= 0 && o.Controlled < 11 && o.Controlled == c.previous.Controlled && o.Height < 10 && o.Ball.Sub(o.Players[o.Team][o.Controlled].Pos).Len() < 4 && o.Ball.Sub(c.previous.Ball).Len() <= 1 && o.Players[o.Team][o.Controlled].Pos.Sub(c.previous.Players[o.Team][o.Controlled].Pos).Len() <= 1 && o.Ball.Sub(o.Players[1-o.Team][0].Pos).Len() > 22 {
		c.trapEvidenceFrames++
	} else {
		c.trapEvidenceFrames = 0
	}
	if c.initialized {
		frames := o.Frame - c.previous.Frame
		if frames > 0 {
			// Original restart placements visibly jump hundreds of pixels. They
			// are new observations, not flights to extrapolate. The deliberately
			// generous 80 pixels per PAL frame also allows fast shots.
			motionReset = o.Ball.Sub(c.previous.Ball).Len() > 80*float64(frames)
			if motionReset {
				c.velocity, c.verticalVelocity = engine.Vec2{}, 0
			} else {
				measured := o.Ball.Sub(c.previous.Ball).Mul(float64(engine.TPS) / float64(frames))
				c.velocity = c.velocity.Mul(0.35).Add(measured.Mul(0.65))
				c.verticalVelocity = c.verticalVelocity*0.35 + (o.Height-c.previous.Height)*float64(engine.TPS)/float64(frames)*0.65
			}
			if o.Controlled == c.previous.Controlled && o.Controlled >= 0 && o.Controlled < 11 {
				measuredPlayer := o.Players[o.Team][o.Controlled].Pos.Sub(c.previous.Players[o.Team][o.Controlled].Pos).Mul(float64(engine.TPS) / float64(frames))
				c.playerVelocity = c.playerVelocity.Mul(0.35).Add(measuredPlayer.Mul(0.65))
				if speed := measuredPlayer.Len(); speed > 80 && speed < 280 {
					c.runningSpeed = c.runningSpeed*0.8 + speed*0.2
				}
			} else {
				c.playerVelocity = engine.Vec2{}
			}
		}
	}
	if o.Controlled != c.lastControlled || o.Phase != c.lastPhase {
		c.trapFrames = 0
		c.ownTouchFrames = 0
		// Automatic player selection may change during a shot's flight. Its
		// short follow-through belongs to the joystick action, not the marker.
		if o.Phase != c.lastPhase || c.pendingShotFrames == 0 && c.aftertouchFrames == 0 && c.shotHoldFrames == 0 {
			c.buttonFrames = 0
			c.aftertouchFrames = 0
			c.shotHoldFrames = 0
		}
		if o.Phase != c.lastPhase || c.macroPhase > 0 && o.Controlled != c.macroPlayer {
			c.macroPhase, c.macroFrames = 0, 0
		}
		c.lastControlled = o.Controlled
		c.lastPhase = o.Phase
	}
	visibleLaunch := c.observeFiredBall(o, motionReset)
	c.previous = o
	c.initialized = true
	forecast := o
	if o.Phase == engine.Playing {
		delay := float64(c.options.ReactionFrames) / engine.TPS
		forecast.Ball = o.Ball.Add(c.velocity.Mul(delay))
		forecast.Height = math.Max(0, o.Height+c.verticalVelocity*delay-0.5*132*delay*delay)
		if o.Height < 8 && c.ownTouchFrames > 0 {
			// A low dribble's recurring visible hops are not an unbounded
			// projectile. Do not invent a header from a six-frame extrapolation.
			forecast.Height = o.Height
		}
		if o.Controlled >= 0 && o.Controlled < 11 {
			forecast.Players[o.Team][o.Controlled].Pos = c.predictSelf(o, current.Frame)
			// A player knows their own most recent direction command before delayed
			// visual feedback arrives. Use that intent to avoid a three-octant reversal.
			if c.lastCommandPlayer == o.Controlled && c.lastMove.Len() > 0.1 && o.Height < 24 && c.ownTouchFrames > 0 {
				forecast.Players[o.Team][o.Controlled].Facing = c.lastMove
			}
		}
	}
	if c.ownTouchFrames > 0 {
		c.ownTouchFrames--
	}
	keeper := o.Players[1-o.Team][0]
	if o.Height < 6 && o.Ball.Sub(keeper.Pos).Len() < 18 && c.velocity.Len() < 25 {
		c.keeperBallFrames++
	} else {
		c.keeperBallFrames = 0
	}
	if c.attackSide == 0 || newAttack {
		c.wingEstablished = false
		c.laneStage, c.laneFrames, c.laneCorrections = 0, 0, 0
		c.attackSide = 1
		if keeper.Pos.X > engine.PitchWidth/2 {
			c.attackSide = -1
		}
	}
	if forecast.Controlled >= 0 && forecast.Controlled < 11 {
		p := forecast.Players[forecast.Team][forecast.Controlled]
		if forecast.Ball.Sub(p.Pos).Len() < 12 && forecast.Height < 8 && c.lastMove.Len() > 0.1 && c.velocity.Dot(c.lastMove) > 0 {
			c.ownTouchFrames = 30
		}
	}
	in := c.choose(forecast)
	// Sudden reversal at a close dribble is the original chip gesture. A skilled
	// dribbler circles sideways to recover a ball that has fallen behind the foot.
	if forecast.Controlled >= 0 && forecast.Controlled < 11 && forecast.Phase == engine.Playing {
		player := forecast.Players[forecast.Team][forecast.Controlled]
		if c.ownTouchFrames > 0 && forecast.Ball.Sub(player.Pos).Len() < 38 && forecast.Height < 8 && in.Move.Dot(player.Facing) < -0.65 {
			clockwise := engine.Vec2{X: -player.Facing.Y, Y: player.Facing.X}
			counter := clockwise.Mul(-1)
			if counter.Dot(in.Move) > clockwise.Dot(in.Move) {
				clockwise = counter
			}
			in.Move = joystick(clockwise)
		}
	}

	if c.options.PursuitGestureFrames > 0 {
		in = c.steadyPursuitInput(in, forecast, current.Frame)
		in = c.avoidSampledAerialReversal(in, forecast, current.Frame)
	} else {
		in = c.avoidAerialReversal(in, forecast, current.Frame)
	}
	label := c.decision
	if label == "" {
		label = "dribble"
	}
	if c.pendingShotFrames > 0 {
		label = "confirm-shot"
	}
	if forecast.Phase != engine.Playing {
		label = "restart"
	} else if c.aftertouchFrames > 0 {
		label = "aftertouch"
		if in.Fire {
			label = "shot"
		}
	} else if c.trapFrames > 0 {
		label = "trap"
	} else if forecast.Height > 8 {
		label = "intercept-aerial"
		if in.Fire {
			label = "header"
		}
	} else if in.Fire {
		label = "fire"
	}
	distance, goalDistance := 0.0, 0.0
	if forecast.Controlled >= 0 && forecast.Controlled < 11 {
		p := forecast.Players[forecast.Team][forecast.Controlled]
		distance = forecast.Ball.Sub(p.Pos).Len()
		goalY := 0.0
		if forecast.Attack > 0 {
			goalY = engine.PitchHeight
		}
		goalDistance = math.Abs(p.Pos.Y - goalY)
	}
	c.diagnostic = Diagnostics{Decision: label, ObservationFrame: o.Frame, Controlled: o.Controlled, ForecastHeight: forecast.Height, ForecastDistance: distance, GoalDistance: goalDistance, OwnTouchFrames: c.ownTouchFrames, AttackSide: c.attackSide, ShotAimX: c.shotAimX, KeeperBallFrames: c.keeperBallFrames}
	c.diagnostic.MotionReset = motionReset
	c.diagnostic.VisibleLaunch = visibleLaunch
	c.diagnostic.FollowThroughFrames = c.visibleCurveFrames
	c.diagnostic.MacroPhase = c.macroPhase
	if c.macroPhase > 0 || c.decision == "fresh-contact-shot" || c.decision == "contest-stationary-ball" || c.decision == "visible-shot-aftertouch" || c.decision == "visible-shot-follow-through" {
		c.diagnostic.Decision = c.decision
	}
	c.rememberFireAttempt(o, forecast, in, current.Frame)
	c.lastFire = in.Fire
	c.lastMove = in.Move
	c.lastCommandPlayer = o.Controlled
	c.commands = append(c.commands, plannedCommand{frame: current.Frame, player: o.Controlled, move: in.Move, fire: in.Fire})
	if len(c.commands) > 40 {
		c.commands = c.commands[len(c.commands)-40:]
	}
	return in
}

// predictSelf uses the player's own issued directions during the visual delay.
// A smoothed six-frame-old velocity predicted continued diagonal movement after
// a turn and caused the repeated overshoot seen in the original v6 capture.
func (c *Controller) predictSelf(o Observation, currentFrame int64) engine.Vec2 {
	position := o.Players[o.Team][o.Controlled].Pos
	for frame := o.Frame; frame < currentFrame; frame++ {
		found := false
		for _, command := range c.commands {
			if command.frame == frame && command.player == o.Controlled {
				if !command.fire {
					position = position.Add(command.move.Mul(c.runningSpeed / engine.TPS))
				}
				found = true
				break
			}
		}
		if !found {
			position = position.Add(c.playerVelocity.Mul(1.0 / engine.TPS))
		}
	}
	return position
}

func (c *Controller) choose(o Observation) engine.Input {
	c.decision = "dribble"
	if o.Controlled < 0 || o.Controlled > 10 {
		return engine.Input{}
	}
	player := o.Players[o.Team][o.Controlled]
	if o.Phase == engine.Playing {
		nearCorner := (o.Ball.X < 24 || o.Ball.X > engine.PitchWidth-24) && (o.Ball.Y < 24 || o.Ball.Y > engine.PitchHeight-24)
		outside := player.Pos.X < 0 || player.Pos.X > engine.PitchWidth || player.Pos.Y < 0 || player.Pos.Y > engine.PitchHeight
		if nearCorner && outside && o.Height < 6 && c.velocity.Len() < 25 {
			c.cornerReadyFrames++
		} else {
			c.cornerReadyFrames = 0
		}
		if c.cornerFireFrames > 0 {
			c.cornerFireFrames--
			c.decision = "corner-confirm"
			return engine.Input{Fire: c.cornerFireFrames > 0}
		}
		if c.cornerReadyFrames >= 6 {
			c.cornerReadyFrames = 0
			c.cornerFireFrames = 34
			c.decision = "corner-confirm"
			return engine.Input{Fire: true}
		}
	}
	if o.Phase == engine.FullTime || o.Phase == engine.Goal {
		return engine.Input{}
	}
	if o.Phase == engine.HalfTime {
		return engine.Input{Fire: o.Frame%25 < 3}
	}
	if o.Phase == engine.Kickoff || o.Phase == engine.SetPiece || o.Phase == engine.Shootout {
		return c.setPiece(o, player)
	}
	goal := engine.Vec2{X: engine.PitchWidth / 2, Y: 0}
	if o.Attack > 0 {
		goal.Y = engine.PitchHeight
	}
	distance := player.Pos.Sub(o.Ball).Len()
	if c.visibleCurveFrames > 0 {
		return c.visibleShotFollowThrough(o, goal)
	}
	if c.macroPhase > 0 {
		return c.trapResetGesture(o)
	}
	if c.shotHoldFrames > 0 {
		if !c.shotConfirmed && c.trapEvidenceFrames >= 2 {
			// The visible ball has stopped at the runner. Releasing north
			// would make the source's trap-release shot without aftertouch.
			// Centered release preserves control before a fresh ordinary nudge.
			c.shotHoldFrames = 0
			c.macroPhase, c.macroFrames, c.macroPlayer = 1, 3, o.Controlled
			return c.trapResetGesture(o)
		}
		c.shotHoldFrames--
		if c.velocity.Len() > 290 && c.velocity.Y*float64(o.Attack) > 120 && o.Ball.Sub(c.shotOrigin).Len() > 20 {
			c.shotConfirmed = true
		}
		direction := c.shotDirection
		c.decision = "held-shot-gesture"
		if c.shotConfirmed {
			c.decision = "held-shot-follow-through"
		}
		return engine.Input{Move: direction, Fire: c.shotHoldFrames > 0}
	}
	if c.buttonFrames > 0 {
		c.buttonFrames--
		return engine.Input{Move: c.heldDirection, Fire: true}
	}
	if c.pendingShotFrames > 0 {
		c.pendingShotFrames--
		// A practiced player follows the fire gesture immediately. Waiting for
		// six-frame-delayed launch confirmation exhausted the original swerve
		// window in the v5 capture. A side gesture is bounded to a few frames.
		c.aftertouchFrames = 10
		c.pendingShotFrames = 0
	}
	if c.aftertouchFrames > 0 {
		c.aftertouchFrames--
		direction := c.shotDirection
		if c.aftertouchFrames >= 7 {
			direction = c.shotAfterDirection
		}
		if c.velocity.Len() > 300 && math.Abs(c.velocity.Y) > 100 && o.Ball.Sub(c.shotOrigin).Len() > 20 {
			remaining := (goal.Y - o.Ball.Y) / c.velocity.Y
			if remaining > 0 && remaining < 2 {
				crossing := o.Ball.X + c.velocity.X*remaining
				if crossing < goal.X-engine.GoalWidth/2+14 || crossing > goal.X+engine.GoalWidth/2-14 {
					// Stop adding curve once delayed visual evidence shows a post
					// becoming the greater risk. No future collision is inspected.
					direction = c.shotDirection
				}
			}
		}
		return engine.Input{Move: direction}
	}
	if c.keeperBallFrames >= 6 {
		c.decision = "await-clearance"
		point := engine.Vec2{X: goal.X + float64(c.attackSide)*70, Y: goal.Y - float64(o.Attack)*190}
		if point.Sub(player.Pos).Len() < 18 {
			return engine.Input{}
		}
		return engine.Input{Move: joystick(point.Sub(player.Pos))}
	}

	if c.trapFrames > 0 {
		c.trapFrames--
		if distance < 24 && c.trapFrames < 6 {
			target := c.passTarget(o, player, goal)
			c.trapFrames = 0
			return engine.Input{Move: joystick(target.Sub(player.Pos))}
		}
		return engine.Input{Move: joystick(o.Ball.Sub(player.Pos)), Fire: true}
	}
	if c.ownTouchFrames == 0 && o.Height < 8 && c.velocity.Len() < 35 {
		// Merely coming within 38 pixels does not mean the runner controls the
		// ball. In the original two-human capture that assumption caused long
		// circles around a stationary opponent. Close the visible gap first.
		direction := joystick(o.Ball.Sub(player.Pos))
		contested := false
		for _, opponent := range o.Players[1-o.Team] {
			if !opponent.Keeper && !opponent.SentOff && opponent.Pos.Sub(o.Ball).Len() < 14 {
				contested = true
				break
			}
		}
		if contested && distance < 30 && c.contestWaitFrames == 0 && !c.lastFire {
			c.decision = "contest-stationary-ball"
			c.contestWaitFrames = 18
			c.buttonFrames = c.options.MinimumButtonFrames - 1
			c.heldDirection = direction
			return engine.Input{Move: direction, Fire: true}
		}
		if distance > 6 || contested {
			c.decision = "acquire-stationary-ball"
			return engine.Input{Move: direction}
		}
	}
	// Intercept the next visible landing or rolling position, instead of chasing
	// the ball's obsolete screen position after the reaction delay.
	target := c.interceptionPoint(o)
	target.X = math.Max(10, math.Min(engine.PitchWidth-10, target.X))
	target.Y = math.Max(10, math.Min(engine.PitchHeight-10, target.Y))
	if distance > 38 {
		c.decision = "intercept"
		incoming := c.ownTouchFrames == 0 && c.velocity.Dot(player.Pos.Sub(o.Ball)) > 0 && c.velocity.Len() > 75 && c.velocity.Dot(player.Facing) < -40
		if incoming && distance < 72 && o.Height < 8 {
			c.trapFrames = 9
			return engine.Input{Move: joystick(target.Sub(player.Pos)), Fire: true}
		}
		return engine.Input{Move: joystick(target.Sub(player.Pos))}
	}
	if o.Height > 23 || o.Height > 8 && c.ownTouchFrames == 0 {
		if c.ownTouchFrames == 0 && distance < 18 && o.Height < 25 && math.Abs(goal.Y-player.Pos.Y) < 210 && math.Abs(o.Ball.X-goal.X) < engine.GoalWidth/2-8 {
			c.buttonFrames = c.options.MinimumButtonFrames - 1
			c.heldDirection = joystick(goal.Sub(player.Pos))
			return engine.Input{Move: c.heldDirection, Fire: true}
		}
		return engine.Input{Move: joystick(target.Sub(player.Pos))}
	}
	towardGoal := goal.Sub(player.Pos)
	shot, possible := c.shotLine(o, player, goal)
	if math.Abs(o.Ball.X-goal.X) < engine.GoalWidth/2-6 {
		// A consistent moderately off-center shot is a stronger baseline than
		// refusing every attempt because a speculative goalkeeper ray overlaps.
		shot, possible = engine.Vec2{Y: float64(o.Attack)}, true
	}
	// A shot is attempted only after the observed ball has moved ahead along the
	// footballer's facing direction. The policy never reads LastTeam/lastTouch.
	ahead := o.Ball.Sub(player.Pos).Dot(player.Facing) > 2
	sameTravel := c.velocity.Dot(player.Facing) > 25
	if possible && math.Abs(towardGoal.Y) < c.options.ShotRange && math.Abs(towardGoal.Y) > 35 && distance < 18 && ahead && sameTravel {
		if c.ownTouchFrames > 0 && player.Facing.Dot(shot) > 0.9 {
			c.buttonFrames = 0
			c.heldDirection = shot
			c.shotDirection = shot
			c.aftertouchFrames = 0
			c.pendingShotFrames = 0
			c.shotHoldFrames, c.shotCurveFrames, c.shotConfirmed = 18, 0, false
			c.shotOrigin = o.Ball
			c.shotAimX = goal.X - float64(c.attackSide)*30
			c.shotAfterDirection = engine.Vec2{X: -float64(c.attackSide)}
			if math.Abs(shot.X) < 0.1 {
				c.shotAfterDirection.X = math.Copysign(1, o.Ball.X-goal.X)
			} else {
				// An already angled launch needs height control before any
				// additional side impulse; v7's long diagonal curve missed wide.
				c.shotAfterDirection = shot
			}
			c.decision = "placed-shot"
			return engine.Input{Move: shot, Fire: true}
		}
		return engine.Input{Move: shot}
	}
	// A rolling ball can sit slightly behind the boot between original
	// fifteen-tick contacts. Keep dribbling forward instead of treating that
	// normal gap as a request to turn around and inadvertently chip.

	// Commit to a wide lane to draw the keeper away from the eventual diagonal
	// goal line. Eight-way steering needs an explicit sideways component: v5's
	// 26/100 vector rounded to north forever and never created its planned lane.
	// v9 is deliberately a simpler original-game baseline: keep a committed
	// straight dribble instead of redirecting the loose ball toward a wing.
	run := engine.Vec2{Y: float64(o.Attack)}
	c.decision = "committed-approach"
	for _, opponent := range o.Players[1-o.Team] {
		delta := opponent.Pos.Sub(player.Pos)
		if opponent.SentOff || opponent.Keeper || delta.Len() > 52 || delta.Dot(run.Unit()) < 8 {
			continue
		}
		if delta.X > 0 {
			run.X -= 1
		} else {
			run.X += 1
		}
	}
	if distance < 10 && c.velocity.Len() < 25 {
		return engine.Input{Move: joystick(run)}
	}
	return engine.Input{Move: joystick(run)}
}

// trapResetGesture is a learned normal-input sequence triggered only by visible
// trap confirmation: center/release, brief forward nudge, then held ordinary
// Fire. Its timings do not read ownership, cooldowns, actor states, or RNG.
func (c *Controller) trapResetGesture(o Observation) engine.Input {
	forward := engine.Vec2{Y: float64(o.Attack)}
	switch c.macroPhase {
	case 1:
		c.decision = "trap-neutral-release"
		c.macroFrames--
		if c.macroFrames == 0 {
			c.macroPhase, c.macroFrames = 2, 2
		}
		return engine.Input{}
	case 2:
		c.decision = "trap-forward-nudge"
		c.macroFrames--
		if c.macroFrames == 0 {
			c.macroPhase, c.macroFrames = 3, 15
		}
		return engine.Input{Move: forward}
	case 3:
		c.decision = "fresh-contact-shot"
		c.macroFrames--
		if c.macroFrames == 0 {
			c.macroPhase = 0
		}
		return engine.Input{Move: forward, Fire: true}
	}
	return engine.Input{}
}

// moderateLane completes a turn before attempting one short boot alignment.
// Chasing a continuously diagonal ball's X in v7 never finished the turn and
// carried it to the touchline. Each correction is followed by a settling run.
func (c *Controller) moderateLane(o Observation, p Footballer, goal engine.Vec2) engine.Vec2 {
	forward := engine.Vec2{Y: float64(o.Attack)}
	if c.laneFrames > 0 {
		c.laneFrames--
		return c.laneMove
	}
	if c.laneStage == 0 {
		desiredX := goal.X + float64(c.attackSide)*20
		if math.Abs(desiredX-p.Pos.X) > 4 {
			return engine.Vec2{X: math.Copysign(1, desiredX-p.Pos.X), Y: forward.Y}
		}
		c.laneStage, c.laneFrames, c.laneMove = 1, 8, forward
		return forward
	}
	if c.laneStage == 2 {
		c.laneStage, c.laneFrames, c.laneMove = 1, 8, forward
		return forward
	}
	if c.laneStage == 1 {
		gap := o.Ball.X - p.Pos.X
		if c.laneCorrections < 2 && math.Abs(gap) > 3 && math.Abs(c.velocity.X) < 65 {
			frames := max(1, min(3, int(math.Round(math.Abs(gap)/(c.runningSpeed*0.707/engine.TPS)))))
			c.laneMove = engine.Vec2{X: math.Copysign(1, gap), Y: forward.Y}
			c.laneFrames, c.laneStage = frames-1, 2
			c.laneCorrections++
			return c.laneMove
		}
		c.laneStage = 3
	}
	return forward
}

func (c *Controller) shotLine(o Observation, p Footballer, goal engine.Vec2) (engine.Vec2, bool) {
	keeper := o.Players[1-o.Team][0]
	bestCost := math.Inf(1)
	best := engine.Vec2{}
	for direction := 0; direction < 8; direction++ {
		d := Movement(direction)
		if d.Y*float64(o.Attack) < 0.5 {
			continue
		}
		distance := (goal.Y - o.Ball.Y) / d.Y
		if distance <= 0 {
			continue
		}
		cross := o.Ball.X + d.X*distance
		if math.Abs(cross-goal.X) > engine.GoalWidth/2-12 {
			continue
		}
		cost := distance*0.015 - math.Abs(cross-keeper.Pos.X)
		// Prefer a path with room beside the visible keeper, not only a point
		// inside the goal mouth. Straight shots through a central keeper were
		// repeatedly caught in the original v4/v5 captures.
		segment := goal.Sub(o.Ball)
		segment.X = cross - o.Ball.X
		fraction := math.Max(0, math.Min(1, keeper.Pos.Sub(o.Ball).Dot(segment)/segment.Dot(segment)))
		clearance := keeper.Pos.Sub(o.Ball.Add(segment.Mul(fraction))).Len()
		if clearance < 14 {
			continue
		}
		cost -= clearance * 0.5
		if cost < bestCost {
			bestCost = cost
			best = d
		}
	}
	return best, best.Len() > 0
}
func (c *Controller) passTarget(o Observation, p Footballer, goal engine.Vec2) engine.Vec2 {
	best := goal
	cost := math.Inf(1)
	for i, teammate := range o.Players[o.Team] {
		if i == p.Index || teammate.Keeper || teammate.SentOff {
			continue
		}
		delta := teammate.Pos.Sub(p.Pos)
		if delta.Len() < 45 || delta.Len() > 300 {
			continue
		}
		score := teammate.Pos.Sub(goal).Len() + delta.Len()*0.25
		for _, opponent := range o.Players[1-o.Team] {
			if opponent.SentOff {
				continue
			}
			segment := delta.Dot(delta)
			fraction := math.Max(0, math.Min(1, opponent.Pos.Sub(p.Pos).Dot(delta)/segment))
			nearest := p.Pos.Add(delta.Mul(fraction))
			if opponent.Pos.Sub(nearest).Len() < 20 {
				score += 350
			}
		}
		if score < cost {
			best = teammate.Pos
			cost = score
		}
	}
	return best
}
func (c *Controller) setPiece(o Observation, p Footballer) engine.Input {
	if o.Phase == engine.Shootout && p.Keeper || o.RestartTeam != o.Team && o.Restart == engine.Penalty {
		direction := joystick(o.Ball.Sub(p.Pos))
		return engine.Input{Move: direction, Fire: true}
	}
	if o.RestartTeam != o.Team {
		return engine.Input{}
	}
	if c.buttonFrames > 0 {
		c.buttonFrames--
		return engine.Input{Move: c.heldDirection, Fire: true}
	}
	if c.lastFire {
		return engine.Input{Move: c.heldDirection}
	}
	direction := engine.Vec2{Y: float64(o.Attack)}
	switch o.Restart {
	case engine.KickOff:
		direction = c.passTarget(o, p, engine.Vec2{X: engine.PitchWidth / 2, Y: engine.PitchHeight/2 + float64(o.Attack)*300}).Sub(p.Pos)
	case engine.ThrowIn:
		direction = c.passTarget(o, p, engine.Vec2{X: engine.PitchWidth / 2, Y: p.Pos.Y + float64(o.Attack)*150}).Sub(p.Pos)
	case engine.Corner:
		direction = engine.Vec2{X: engine.PitchWidth / 2, Y: math.Max(70, math.Min(engine.PitchHeight-70, o.Ball.Y))}.Sub(o.Ball)
	case engine.Penalty:
		direction.X = 0.707
		direction.Y = float64(o.Attack) * 0.707
	}
	c.heldDirection = joystick(direction)
	c.buttonFrames = 3
	if o.Restart == engine.Corner {
		c.buttonFrames = 8
	}
	return engine.Input{Move: c.heldDirection, Fire: true}
}

// Diagnostics describes a decision using only the policy's delayed perception.
// The original capture harness stores these alongside requested joystick input.
type Diagnostics struct {
	Decision            string  `json:"decision"`
	ObservationFrame    int64   `json:"observation_frame"`
	Controlled          int     `json:"controlled"`
	ForecastHeight      float64 `json:"forecast_height"`
	ForecastDistance    float64 `json:"forecast_distance"`
	GoalDistance        float64 `json:"goal_distance"`
	OwnTouchFrames      int     `json:"own_touch_frames"`
	AttackSide          int     `json:"attack_side"`
	ShotAimX            float64 `json:"shot_aim_x"`
	KeeperBallFrames    int     `json:"keeper_ball_frames"`
	MacroPhase          int     `json:"macro_phase"`
	MotionReset         bool    `json:"motion_reset"`
	VisibleLaunch       bool    `json:"visible_launch"`
	FollowThroughFrames int     `json:"follow_through_frames"`
}

func (c *Controller) Diagnostics() Diagnostics { return c.diagnostic }
