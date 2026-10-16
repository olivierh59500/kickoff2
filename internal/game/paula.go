package game

import (
	"encoding/binary"
	"fmt"
)

const paulaPALClock = uint64(3546895)

// paulaSample retains the original signed 8-bit payload and its AUDxPER word.
// AUDxLEN receives size>>1 words, so an odd trailing byte never reaches DMA.
type paulaSample struct {
	period      uint16
	sourceBytes int
	data        []byte
}

func decodePaulaSamples(data []byte) ([]paulaSample, error) {
	if len(data) < 6 {
		return nil, fmt.Errorf("short Paula sample bundle")
	}
	footer := int(binary.BigEndian.Uint32(data))
	if footer < 4 || footer > len(data)-2 {
		return nil, fmt.Errorf("invalid Paula footer")
	}
	count := int(binary.BigEndian.Uint16(data[footer:]))
	if count <= 0 || count > (len(data)-footer-2)/4 {
		return nil, fmt.Errorf("invalid Paula sample count")
	}
	samples := make([]paulaSample, count)
	for i := range samples {
		at := int(binary.BigEndian.Uint32(data[footer+2+i*4:]))
		if at < 4 || at > footer-6 {
			return nil, fmt.Errorf("sample%d header out of bounds", i+1)
		}
		period := binary.BigEndian.Uint16(data[at:])
		size := int(binary.BigEndian.Uint32(data[at+2:]))
		if period < 124 || size < 2 || size > footer-at-6 {
			return nil, fmt.Errorf("sample%d payload out of bounds", i+1)
		}
		dmaBytes := size &^ 1
		samples[i] = paulaSample{period: period, sourceBytes: size, data: append([]byte(nil), data[at+6:at+6+dmaBytes]...)}
	}
	return samples, nil
}

type paulaSegment struct {
	sample               int
	period, volume       uint16
	startClock, endClock uint64
}

// paulaSequence translates the pointer/counter decoder at 0xca 8c..0xcbd 0.
// Its negative counter repeats the next sample, not the preceding sample.
type paulaSequence struct {
	commands      []int16
	cursor        int
	repeatCounter int16
	customPeriod  uint16
	volume        uint16
	clock         uint64
	finished      bool
}

func newPaulaSequence(commands []int16) *paulaSequence {
	return &paulaSequence{commands: append([]int16(nil), commands...), volume: 40}
}

func (s *paulaSequence) next(samples []paulaSample) (paulaSegment, bool, error) {
	if s.finished {
		return paulaSegment{}, false, nil
	}
	for operations := 0; operations < 4096; operations++ {
		if s.cursor >= len(s.commands) {
			return paulaSegment{}, false, fmt.Errorf("unterminated original sound sequence")
		}
		at := s.cursor
		s.repeatCounter++
		if s.repeatCounter >= 0 {
			s.cursor++
		}
		command := s.commands[at]
		switch {
		case command == 0:
			s.finished = true
			return paulaSegment{}, false, nil
		case command == -1:
			// No shipped game cue uses this branch. The source resets its
			// pointer but then indexes outside the normal sample directory;
			// treating it as a musical loop would invent unsupported behavior.
			return paulaSegment{}, false, fmt.Errorf("unverified original -1 sound command")
		case command < -1:
			s.repeatCounter = command
			continue
		case command >= 200:
			s.customPeriod = uint16(command - 200)
			continue
		case command >= 130:
			s.volume = min(64, uint16(command-130))
			continue
		case command == 100:
			return paulaSegment{}, false, fmt.Errorf("invalid original sound sample command100")
		}
		index, period := int(command)-1, uint16(0)
		if command >= 101 {
			index = int(command) - 101
			period = s.customPeriod
		}
		if index < 0 || index >= len(samples) {
			return paulaSegment{}, false, fmt.Errorf("original sound sample%d is unavailable", index+1)
		}
		if command < 100 {
			period = samples[index].period
		}
		if period == 0 {
			return paulaSegment{}, false, fmt.Errorf("custom sound sample has no nonzero period")
		}
		segment := paulaSegment{sample: index, period: period, volume: s.volume, startClock: s.clock}
		segment.endClock = segment.startClock + uint64(len(samples[index].data))*uint64(period)
		if segment.endClock > paulaPALClock*60 {
			return paulaSegment{}, false, fmt.Errorf("sound sequence exceeds its one-minute rendering limit")
		}
		s.clock = segment.endClock
		return segment, true, nil
	}
	return paulaSegment{}, false, fmt.Errorf("sound sequence contains no bounded sample command")
}

func compilePaulaSequence(commands []int16, samples []paulaSample) ([]paulaSegment, error) {
	sequence := newPaulaSequence(commands)
	var segments []paulaSegment
	for count := 0; count < 4096; count++ {
		segment, active, err := sequence.next(samples)
		if err != nil {
			return nil, err
		}
		if !active {
			return segments, nil
		}
		segments = append(segments, segment)
	}
	return nil, fmt.Errorf("sound sequence exceeds its sample-command limit")
}

// renderPaulaSequence samples the held 8-bit DMA value against one cumulative
// master-clock timeline, avoiding separately rounded sample boundaries. Hardware
// channels 0/3 feed left and 1/2 feed right; channel-1 supplies the centered decoder
// compatibility view. Analog filtering and cycle-level DMA bus effects are not
// emulated by this portable PCM renderer.
func renderPaulaSequence(segments []paulaSegment, samples []paulaSample, channel int) []byte {
	if len(segments) == 0 {
		return nil
	}
	end := segments[len(segments)-1].endClock
	frames := int((end*audioRate + paulaPALClock - 1) / paulaPALClock)
	pcm := make([]byte, frames*4)
	current := 0
	for frame := 0; frame < frames; frame++ {
		clock := uint64(frame) * paulaPALClock / audioRate
		for current+1 < len(segments) && clock >= segments[current].endClock {
			current++
		}
		segment := segments[current]
		index := int((clock - segment.startClock) / uint64(segment.period))
		source := samples[segment.sample].data
		if index >= len(source) {
			continue
		}
		// Two hardware channels can sum on each side. Half-scale output keeps
		// their full-volume sum within portable signed 16-bit PCM headroom.
		value := int16(int32(int8(source[index])) * 128 * int32(segment.volume) / 64)
		if channel == -1 || channel == 0 || channel == 3 {
			binary.LittleEndian.PutUint16(pcm[frame*4:], uint16(value))
		}
		if channel == -1 || channel == 1 || channel == 2 {
			binary.LittleEndian.PutUint16(pcm[frame*4+2:], uint16(value))
		}
	}
	return pcm
}
