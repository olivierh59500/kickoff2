package game

import "github.com/olivierh59500/kickoff2/internal/engine"

func cornerChoice(move engine.Vec2) int {
	if move.Len() < 0.01 {
		return 0
	}
	return engine.OriginalDirectionIndex(move) + 1
}
