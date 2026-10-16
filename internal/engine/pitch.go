package engine

// OriginalIndex converts the saved native preference to the original setup
// and replay word. The original orders Wet before Soggy; native saves do not.
func (p Pitch) OriginalIndex() uint16 {
	switch p {
	case Wet:
		return 1
	case Soggy:
		return 2
	case Artificial:
		return 3
	case RandomPitch:
		return 4
	default:
		return 0
	}
}

// PitchFromOriginalIndex interprets an original menu, random or replay word.
func PitchFromOriginalIndex(index uint16) Pitch {
	switch index {
	case 1:
		return Wet
	case 2:
		return Soggy
	case 3:
		return Artificial
	case 4:
		return RandomPitch
	default:
		return Normal
	}
}
