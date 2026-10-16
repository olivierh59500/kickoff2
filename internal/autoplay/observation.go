// Package autoplay drives the game through the original joystick and fire button.
// It deliberately receives visual observations rather than private engine state.
package autoplay

import (
	"github.com/olivierh59500/kickoff2/internal/engine"
	"math"
)

// Footballer contains information visible in the field or scanner.
type Footballer struct {
	Team, Index     int
	Pos, Facing     engine.Vec2
	Keeper, SentOff bool
}

// Observation contains visible positions and announcements, without the RNG,
// hidden attributes, contact cooldowns, or the engine's possession identifier.
type Observation struct {
	Frame                    int64
	Team, Controlled, Attack int
	Phase                    engine.Phase
	Restart                  engine.RestartKind
	RestartTeam              int
	Ball                     engine.Vec2
	Height                   float64
	Players                  [2][11]Footballer
	Score                    [2]int
}

// ObserveMatch quantizes remote scanner positions more coarsely than the active
// field. A controller can see the ball and the controlled player's position but
// cannot read velocities, hidden skill values, or future random decisions.
func ObserveMatch(match *engine.Match, team int) Observation {
	o := Observation{Frame: match.Tick, Team: team, Controlled: match.Teams[team].Controlled, Attack: match.AttackDirection(team), Phase: match.Phase, Restart: match.Restart.Kind, RestartTeam: match.Restart.Team, Ball: rounded(match.Ball.Pos, 1), Height: math.Round(match.Ball.Height)}
	if match.Teams[team].ControlledOpposing {
		// The primary marker can temporarily point outside this roster. It
		// does not make the same shirt number on this team controllable.
		o.Controlled = -1
	}
	// The original corner panel is a visible modal screen even though the
	// source match action remains Playing. Expose that presentation state so
	// the same human controller can confirm its choice through ordinary Fire.
	if match.OriginalCornerSelectorActive() {
		o.Phase, o.Restart = engine.SetPiece, engine.Corner
	}
	if kind, taking, shown := match.OriginalFoulRestartPresentation(); shown {
		o.Phase, o.Restart, o.RestartTeam = engine.SetPiece, kind, taking
	}
	if match.OriginalPenaltyContestActive() {
		o.Phase, o.Restart, o.RestartTeam = engine.Shootout, engine.Penalty, match.Restart.Team
	}
	camera := match.OriginalCameraOffset()
	for t, side := range match.Teams {
		o.Score[t] = side.Score
		for i, p := range side.Players {
			resolution := 1.0
			facing := rounded(p.Facing, 0.01)
			if !VisibleInCamera(p.Pos, camera) {
				resolution = 16
				facing = engine.Vec2{Y: float64(match.AttackDirection(t))}
			}
			o.Players[t][i] = Footballer{t, i, rounded(p.Pos, resolution), facing, p.Keeper, p.SentOff}
		}
	}
	return o
}

// VisibleInCamera uses the same conservative view interior for both adapters;
// scanner-only footballers never expose their hidden sprite direction.
func VisibleInCamera(position, camera engine.Vec2) bool {
	return position.X >= camera.X+8 && position.X < camera.X+312 && position.Y >= camera.Y+8 && position.Y < camera.Y+216
}
func rounded(v engine.Vec2, step float64) engine.Vec2 {
	return engine.Vec2{X: math.Round(v.X/step) * step, Y: math.Round(v.Y/step) * step}
}

// Direction is the exact original joystick octant: north, northeast, east,
// southeast, south, southwest, west, northwest. Minus one means centered.
func Direction(move engine.Vec2) int {
	if move.Len() < 0.1 {
		return -1
	}
	return (int(math.Round((math.Atan2(move.Y, move.X)+math.Pi/2)/(math.Pi/4))) + 8) % 8
}

// Movement reproduces the original signed movement table's diagonal ratio.
func Movement(direction int) engine.Vec2 {
	table := [8]engine.Vec2{{X: 0, Y: -1}, {X: 0.707, Y: -0.707}, {X: 1}, {X: 0.707, Y: 0.707}, {Y: 1}, {X: -0.707, Y: 0.707}, {X: -1}, {X: -0.707, Y: -0.707}}
	if direction < 0 || direction >= len(table) {
		return engine.Vec2{}
	}
	return table[direction]
}
func joystick(move engine.Vec2) engine.Vec2 { return Movement(Direction(move)) }
