package replaydata

import (
	"encoding/binary"
	"fmt"
)

// Recorder follows the live FCA6/EEA0/F93C writers. It retains exactly24KiB
// rather than dropping frames after a fixed duration. Every source write is
// a six-byte record, including each third of an eighteen-byte name payload.
type Recorder struct {
	ring   [RingSize]byte
	offset uint32
	frames uint64
}

func NewRecorder() *Recorder { return &Recorder{} }

// ResumeRecorder copies an explicitly supplied initial buffer. It does not
// import simulation state or consume later observations during recording.
func ResumeRecorder(clip *Clip) (*Recorder, error) {
	if clip == nil || clip.WriteOffset()%6 != 0 {
		return nil, fmt.Errorf("live recording requires a six-byte-aligned source cursor")
	}
	return &Recorder{ring: clip.Ring, offset: clip.WriteOffset()}, nil
}

func (recorder *Recorder) writeRecord(first, second, third int16) {
	for index, word := range [3]int16{first, second, third} {
		binary.BigEndian.PutUint16(recorder.ring[int(recorder.offset)+index*2:], uint16(word))
	}
	recorder.offset += 6
	if recorder.offset == RingSize {
		recorder.offset = 0
	}
}

// AppendFrame preserves the live writer's explicit marker and command order.
// It does not normalize a frame to the separate decoder's24-slot boundary.
func (recorder *Recorder) AppendFrame(frame Frame) error {
	for _, command := range frame.Commands {
		if err := validateCommand(command); err != nil {
			return err
		}
		if command.X == FrameMark {
			return fmt.Errorf("live command X collides with the frame marker")
		}
	}
	recorder.writeRecord(FrameMark, frame.FocusX, frame.FocusY)
	for _, command := range frame.Commands {
		recorder.writeRecord(command.X, command.Y, command.Code)
		if command.Code < -2 {
			for index := 0; index < 3; index++ {
				recorder.writeRecord(int16(binary.BigEndian.Uint16(command.Name[index*6:])), int16(binary.BigEndian.Uint16(command.Name[index*6+2:])), int16(binary.BigEndian.Uint16(command.Name[index*6+4:])))
			}
		}
	}
	recorder.frames++
	return nil
}

func (recorder *Recorder) RecordedFrames() uint64 { return recorder.frames }

// Snapshot copies the completed stream and adds the next FCA6 target, matching
// an A474 entry boundary. The original reader discards its terminating word;
// this target lets the last complete command frame remain playable.
// Taking a snapshot never advances or changes the ongoing recorder.
func (recorder *Recorder) Snapshot(metadata Metadata, nextX, nextY int16) (*Clip, error) {
	copy := *recorder
	copy.writeRecord(FrameMark, nextX, nextY)
	clip := &Clip{Ring: copy.ring}
	if err := clip.SetMetadata(metadata); err != nil {
		return nil, err
	}
	binary.BigEndian.PutUint32(clip.Header[74:], copy.offset)
	binary.BigEndian.PutUint16(clip.Header[82:], uint16(RingSize-copy.offset))
	return clip, nil
}
