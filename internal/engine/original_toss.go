package engine

// OriginalToss translates7874/7724. EndChoice is zero for up pitch and one for
// down pitch, relative to the winning team. The other team takes the kickoff.
type OriginalToss struct {
	Winner, EndChoice, InitialKickoff int
	HumanChoice, TeamADown            bool
}

// NewOriginalToss consumes only the source's explicit toss/CPU-choice words.
// Original busy-loop input entropy is separate from deterministic native menus.
func NewOriginalToss(human [2]bool, state *[3]uint16) OriginalToss {
	toss := OriginalToss{Winner: -1}
	if !human[0] && !human[1] {
		return toss
	}
	toss.Winner = 1 - int(OriginalRandomWord(state)&1)
	toss.InitialKickoff = 1 - toss.Winner
	toss.HumanChoice = human[toss.Winner]
	choice := 0
	if !toss.HumanChoice {
		choice = int(OriginalRandomWord(state) & 1)
	}
	toss.ChooseEnd(choice)
	return toss
}

// ChooseEnd changes the winner's end without consuming another random word.
func (t *OriginalToss) ChooseEnd(choice int) {
	t.EndChoice = clampInt(choice, 0, 1)
	t.TeamADown = t.Winner >= 0 && t.EndChoice^t.Winner != 0
}
