package engine

// MatchPreparation retains the ordinary wind, tactics, referee and RNG setup
// while the player chooses ends. No football update runs before Start.
type MatchPreparation struct {
	match   *Match
	tactics [2]OriginalTacticState
	toss    *OriginalToss
	started bool
}

// OriginalPitchChoice follows78DE: the random option is sampled at match setup,
// before wind selection. Fixed ground choices do not consume a random word.
func OriginalPitchChoice(option Pitch, state *[3]uint16) Pitch {
	if option >= RandomPitch {
		return PitchFromOriginalIndex(OriginalRandomWord(state) % 4)
	}
	return Pitch(clampInt(int(option), 0, 3))
}

// PrepareMatch performs the same deterministic setup prefix as New.
func PrepareMatch(config Config) *MatchPreparation { return prepareMatch(config, nil) }

// PrepareMatchWithOriginalRandomState continues the tournament/menu generator.
func PrepareMatchWithOriginalRandomState(config Config, state [3]uint16) *MatchPreparation {
	return prepareMatch(config, &state)
}

func (p *MatchPreparation) Config() Config         { return p.match.Config }
func (p *MatchPreparation) Referee() Referee       { return p.match.Referee }
func (p *MatchPreparation) RandomState() [3]uint16 { return p.match.originalRandomState }
func (p *MatchPreparation) WindVector() (int32, int32) {
	return p.match.originalWindX, p.match.originalWindY
}

// AdvanceMenuRandom accounts for6B4E's first released-button poll. Native
// menus sample once on entry, without reproducing the original busy-loop time.
func (p *MatchPreparation) AdvanceMenuRandom() {
	if p.started {
		return
	}
	OriginalRandomWord(&p.match.originalRandomState)
}

// Toss draws once. Reopening a store screen or redrawing a menu cannot reroll it.
func (p *MatchPreparation) Toss() OriginalToss {
	if p.toss == nil {
		if p.started {
			return OriginalToss{Winner: -1}
		}
		toss := NewOriginalToss(p.match.Config.Human, &p.match.originalRandomState)
		p.toss = &toss
	}
	return *p.toss
}
