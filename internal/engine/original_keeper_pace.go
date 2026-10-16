package engine

// originalKeeperCarryStep translates C882. A leading keeper consumes time;
// a trailing keeper runs faster, and uses full pace in the final minute.
func (m *Match) originalKeeperCarryStep(team int, p *Player) int {
	pace := originalPace(p)
	difference := m.Teams[team].Score - m.Teams[1-team].Score
	if difference == 0 {
		return pace - 60
	}
	difference *= 8
	if difference > 0 {
		return pace - (difference + 60)
	}
	limit := m.Config.HalfSeconds * TPS
	if m.Half%2 == 0 {
		limit *= 2
	}
	if int64(limit)-int64(m.originalClock) < 3000 {
		return pace
	}
	return pace - (difference + 30)
}
