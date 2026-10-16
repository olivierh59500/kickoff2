package engine

// OriginalRandomWord translates the three sequential rotate-with-extend words
// at 0x16922. The first state word is the returned random value. Passing actual
// source words allows independent launch-state comparisons without inventing a
// relationship between a modern configuration seed and Amiga menu timing.
func OriginalRandomWord(state *[3]uint16) uint16 {
	carry := ((state[1] >> 3) ^ state[2]) & 1
	for i := range state {
		next := state[i] >> 15
		state[i] = state[i]<<1 | carry
		carry = next
	}
	return state[0]
}

// SetOriginalRandomState restores the three words observed at 0x1a5bc. It is
// useful for original-state imports and deterministic save-state comparisons.
func (m *Match) SetOriginalRandomState(state [3]uint16) { m.originalRandomState = state }

// OriginalRandomState returns a copy of the current three-word generator state.
func (m *Match) OriginalRandomState() [3]uint16 { return m.originalRandomState }
