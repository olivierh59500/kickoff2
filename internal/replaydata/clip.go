// Package replaydata reads and writes the original Kick Off 2 replay records.
// It retains screen commands; decoding never resumes a live football match.
package replaydata

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

const (
	HeaderSize = 84
	RingSize   = 0x6000
	RecordSize = HeaderSize + RingSize
	FrameMark  = int16(-30000)
)

// Kit retains the original style index and two twelve-bit hardware colors.
// Shirt and Shorts are color words, not indices into the native kit editor.
type Kit struct {
	Style, Shirt, Shorts uint16
}

// Metadata contains the six ten-byte fields and rendering setup at 981C/9868.
type Metadata struct {
	Fields [6]string
	Pitch  uint16 // Original order: Normal, Wet, Soggy, Artificial.
	Kits   [2]Kit
}

// Clip preserves the entire source header, circular buffer and ignored trailer.
// The four header bytes at 78..81 have no role in the observed playback decoder.
type Clip struct {
	Header   [HeaderSize]byte
	Ring     [RingSize]byte
	Trailing []byte
}

// Command stores one original screen-space triple. Nonnegative codes are global
// sprite indices; -1/-2 are controller masks. Other negative codes carry the
// eighteen raw bytes following a player-name change, including unused bytes
// after its NUL. Those bytes must survive a lossless import/export.
type Command struct {
	X, Y, Code int16
	Name       [18]byte
}

// Frame retains the recorded focus target rather than guessing a camera offset.
// Complete means a marker or the 24th sprite slot closed the original frame.
// The reader can terminate midway through the newest recorded command.
type Frame struct {
	FocusX, FocusY int16
	Commands       []Command
	Complete       bool
}

func decodeText(data []byte) string {
	if end := bytes.IndexByte(data, 0); end >= 0 {
		data = data[:end]
	}
	runes := make([]rune, len(data))
	for index, value := range data {
		runes[index] = rune(value)
	}
	return string(runes)
}

// NameText interprets only the NUL-terminated Latin-1 name within the raw bytes.
func (command Command) NameText() string { return decodeText(command.Name[:]) }

func (clip *Clip) WriteOffset() uint32 { return binary.BigEndian.Uint32(clip.Header[74:78]) }
func (clip *Clip) Remaining() uint16   { return binary.BigEndian.Uint16(clip.Header[82:84]) }

func (clip *Clip) Metadata() Metadata {
	var result Metadata
	for index := range result.Fields {
		result.Fields[index] = decodeText(clip.Header[index*10 : index*10+10])
	}
	result.Pitch = binary.BigEndian.Uint16(clip.Header[60:62])
	for team := range result.Kits {
		offset := 62 + team*6
		result.Kits[team] = Kit{binary.BigEndian.Uint16(clip.Header[offset:]),
			binary.BigEndian.Uint16(clip.Header[offset+2:]), binary.BigEndian.Uint16(clip.Header[offset+4:])}
	}
	return result
}

// SetMetadata updates only the understood header fields. A failed validation
// leaves every existing byte intact, including the source ring and spare words.
func (clip *Clip) SetMetadata(metadata Metadata) error {
	if metadata.Pitch > 3 {
		return fmt.Errorf("original replay pitch %d is outside 0..3", metadata.Pitch)
	}
	var fields [60]byte
	copy(fields[:], clip.Header[:60])
	for index, field := range metadata.Fields {
		length := 0
		for _, value := range field {
			if value <= 0 || value > 255 || length >= 9 {
				return fmt.Errorf("original replay field %d must fit nine nonzero Latin-1 bytes", index)
			}
			fields[index*10+length] = byte(value)
			length++
		}
		fields[index*10+length] = 0
	}
	for _, kit := range metadata.Kits {
		if kit.Style > 5 || kit.Shirt > 0xfff || kit.Shorts > 0xfff {
			return fmt.Errorf("invalid original replay style or twelve-bit kit color")
		}
	}
	copy(clip.Header[:60], fields[:])
	binary.BigEndian.PutUint16(clip.Header[60:], metadata.Pitch)
	for team, kit := range metadata.Kits {
		offset := 62 + team*6
		binary.BigEndian.PutUint16(clip.Header[offset:], kit.Style)
		binary.BigEndian.PutUint16(clip.Header[offset+2:], kit.Shirt)
		binary.BigEndian.PutUint16(clip.Header[offset+4:], kit.Shorts)
	}
	return nil
}

func (clip *Clip) validate() error {
	offset := clip.WriteOffset()
	if offset >= RingSize || offset&1 != 0 || uint32(clip.Remaining()) != RingSize-offset {
		return fmt.Errorf("inconsistent original replay ring offset/count")
	}
	for index := 0; index < 6; index++ {
		if bytes.IndexByte(clip.Header[index*10:index*10+10], 0) < 0 {
			return fmt.Errorf("unterminated original replay metadata field %d", index)
		}
	}
	metadata := clip.Metadata()
	if metadata.Pitch > 3 {
		return fmt.Errorf("invalid original replay pitch")
	}
	for _, kit := range metadata.Kits {
		if kit.Style > 5 || kit.Shirt > 0xfff || kit.Shorts > 0xfff {
			return fmt.Errorf("invalid original replay kit")
		}
	}
	return nil
}

// Parse reads the block loaded by 962A: an 84-byte header followed by 24 KiB.
// The source ignores additional file bytes; preserve a bounded trailer exactly.
func Parse(data []byte) (*Clip, error) {
	if len(data) < RecordSize || len(data) > 65536 {
		return nil, fmt.Errorf("original replay size %d is outside %d..65536", len(data), RecordSize)
	}
	clip := &Clip{Trailing: bytes.Clone(data[RecordSize:])}
	copy(clip.Header[:], data[:HeaderSize])
	copy(clip.Ring[:], data[HeaderSize:RecordSize])
	if err := clip.validate(); err != nil {
		return nil, err
	}
	return clip, nil
}

func (clip *Clip) MarshalBinary() ([]byte, error) {
	if err := clip.validate(); err != nil {
		return nil, err
	}
	if len(clip.Trailing) > 65536-RecordSize {
		return nil, fmt.Errorf("original replay trailer exceeds its size bound")
	}
	result := make([]byte, 0, RecordSize+len(clip.Trailing))
	result = append(result, clip.Header[:]...)
	result = append(result, clip.Ring[:]...)
	return append(result, clip.Trailing...), nil
}

// WordRead exposes the exact 98D6/F16C read, wrap and subsequent pointer compare.
// End is tested after the read, so the terminating word is not consumed by the
// original playback caller even though D0 and its local cursor have advanced.
type WordRead struct {
	Word              uint16
	Offset, Remaining uint16
	End               bool
}

func (clip *Clip) ReadWord(offset, remaining uint16) (WordRead, error) {
	if offset >= RingSize || offset&1 != 0 || int(remaining) != RingSize-int(offset) {
		return WordRead{}, fmt.Errorf("invalid original replay word-reader cursor")
	}
	result := WordRead{Word: binary.BigEndian.Uint16(clip.Ring[offset:]),
		Offset: offset + 2, Remaining: remaining - 2}
	if result.Remaining == 0 {
		result.Offset, result.Remaining = 0, RingSize
	}
	result.End = uint32(result.Offset) == clip.WriteOffset()
	return result, nil
}

// Frames decodes only words accepted by the original caller. Leading overwritten
// records are skipped until a marker. An incomplete newest target/triple/name is
// a normal ring ending, not permission to read beyond the saved buffer.
func (clip *Clip) Frames() ([]Frame, error) {
	if err := clip.validate(); err != nil {
		return nil, err
	}
	offset, remaining := uint16(clip.WriteOffset()), clip.Remaining()
	words := make([]int16, 0, RingSize/2-1)
	for count := 0; count < RingSize/2; count++ {
		read, err := clip.ReadWord(offset, remaining)
		if err != nil {
			return nil, err
		}
		offset, remaining = read.Offset, read.Remaining
		if read.End {
			break
		}
		words = append(words, int16(read.Word))
	}
	var result []Frame
	var current *Frame
	sprites, masks := 0, 0
	implicitTarget := false
	for cursor := 0; cursor < len(words); {
		// 9A26 falls directly into 9970 after sprite slot 24. That path
		// reads two focus words without first consuming another marker.
		if implicitTarget {
			if cursor+2 > len(words) {
				break
			}
			current = &Frame{FocusX: words[cursor], FocusY: words[cursor+1]}
			cursor += 2
			sprites, masks, implicitTarget = 0, 0, false
			continue
		}
		x := words[cursor]
		cursor++
		if x == FrameMark {
			if current != nil {
				current.Complete = true
				result = append(result, *current)
				current = nil
			}
			if cursor+2 > len(words) {
				break
			}
			current = &Frame{FocusX: words[cursor], FocusY: words[cursor+1]}
			cursor += 2
			sprites, masks = 0, 0
			continue
		}
		if current == nil {
			continue
		}
		if cursor+2 > len(words) {
			break
		}
		command := Command{X: x, Y: words[cursor], Code: words[cursor+1]}
		cursor += 2
		if command.Code >= 0 {
			sprites++
		} else if command.Code >= -2 {
			masks++
		} else {
			if cursor+9 > len(words) {
				break
			}
			for index := 0; index < 9; index++ {
				binary.BigEndian.PutUint16(command.Name[index*2:], uint16(words[cursor+index]))
			}
			cursor += 9
		}
		if err := validateCommand(command); err != nil {
			return nil, fmt.Errorf("original replay frame %d: %w", len(result), err)
		}
		if sprites > 24 || masks > 4 {
			return nil, fmt.Errorf("original replay frame exceeds its 24 sprite/four mask slots")
		}
		current.Commands = append(current.Commands, command)
		if sprites == 24 {
			current.Complete = true
			result = append(result, *current)
			current, implicitTarget = nil, true
		}
	}
	if current != nil {
		result = append(result, *current)
	}
	return result, nil
}

func validateCommand(command Command) error {
	switch {
	case command.Code >= 0:
		// EF0C clips before consulting the sprite table. Off-screen triples
		// still consume a slot even when their index would not be drawable.
		visible := command.X >= 0 && command.X < 335 && command.Y >= 0 && command.Y < 239
		if visible && command.Code >= 820 {
			return fmt.Errorf("invalid original replay sprite command")
		}
	case command.Code >= -2:
		// Controller masks use the raw ground coordinates. A video interrupt
		// after sprite clipping can move these outside that earlier viewport.
	default:
		if bytes.IndexByte(command.Name[:], 0) < 0 {
			return fmt.Errorf("unterminated original replay player name")
		}
	}
	return nil
}

// New builds a stream accepted by the original word reader. Ordinary records
// occupy six bytes; filling all 24 sprite slots implicitly begins the next
// two-word target. A final boundary closes the newest complete frame before the
// terminal word. Byte capacity determines duration, not a fixed frame count.
func New(metadata Metadata, frames []Frame) (*Clip, error) {
	clip := &Clip{}
	if err := clip.SetMetadata(metadata); err != nil {
		return nil, err
	}
	offset := 0
	write := func(words ...int16) {
		for _, word := range words {
			binary.BigEndian.PutUint16(clip.Ring[offset:], uint16(word))
			offset = (offset + 2) % RingSize
		}
	}
	marker := true
	for _, frame := range frames {
		if marker {
			write(FrameMark)
		}
		write(frame.FocusX, frame.FocusY)
		sprites, masks := 0, 0
		for index, command := range frame.Commands {
			if err := validateCommand(command); err != nil {
				return nil, err
			}
			if command.X == FrameMark {
				return nil, fmt.Errorf("a replay command cannot contain the frame marker as X")
			}
			if command.Code >= 0 {
				sprites++
			} else if command.Code >= -2 {
				masks++
			}
			if sprites > 24 || masks > 4 {
				return nil, fmt.Errorf("original replay frame exceeds its sprite/mask capacity")
			}
			if sprites == 24 && index != len(frame.Commands)-1 {
				return nil, fmt.Errorf("the 24th original sprite slot terminates its frame")
			}
			write(command.X, command.Y, command.Code)
			if command.Code < -2 {
				for index := 0; index < 9; index++ {
					write(int16(binary.BigEndian.Uint16(command.Name[index*2:])))
				}
			}
		}
		marker = sprites < 24
	}
	if len(frames) > 0 {
		last := frames[len(frames)-1]
		if marker {
			write(FrameMark)
		}
		write(last.FocusX, last.FocusY)
	}
	binary.BigEndian.PutUint32(clip.Header[74:], uint32(offset))
	binary.BigEndian.PutUint16(clip.Header[82:], uint16(RingSize-offset))
	return clip, nil
}
