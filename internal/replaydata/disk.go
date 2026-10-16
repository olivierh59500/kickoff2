package replaydata

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"path"
	"strconv"
	"strings"
)

const DataDiskSize = 1760 * 512

// DiskFile describes the custom 15654..15B7C flat directory. These data disks
// are distinct from AmigaDOS OFS/FFS volumes despite using standard ADF sectors.
type DiskFile struct {
	Name string
	Data []byte
}

// IsClipFilename recognizes standalone Golden Shots and numbered match pieces.
// A .rep itself is an index marker and contains no circular replay record.
func IsClipFilename(name string) bool {
	extension := strings.ToLower(path.Ext(name))
	if extension == ".grp" {
		return true
	}
	if !strings.HasPrefix(extension, ".z") || len(extension) < 3 {
		return false
	}
	_, err := strconv.ParseUint(extension[2:], 10, 15)
	return err == nil
}

// ParseDataDisk reads the original 32-entry directory at sector two. Empty
// .rep index files are real entries and can share a sector with the next file.
// The data.disk marker is written by the original data-disk initialization.
func ParseDataDisk(data []byte) ([]DiskFile, error) {
	if len(data) != DataDiskSize {
		return nil, fmt.Errorf("original replay data disk must contain 1760 sectors")
	}
	var result []DiskFile
	known := make(map[string]bool)
	type extent struct{ start, end int }
	var occupied []extent
	for index := 0; index < 32; index++ {
		record := data[1024+index*16 : 1024+(index+1)*16]
		packed := binary.BigEndian.Uint32(record[12:])
		if packed == 0 {
			continue
		}
		name := decodeText(record[:12])
		if err := validDiskName(name); err != nil {
			return nil, err
		}
		key := FilenameKey(name)
		if known[key] {
			return nil, fmt.Errorf("duplicate original data-disk filename %q", name)
		}
		known[key] = true
		sector, size := int(packed>>20), int(packed&0xfffff)
		start, end := sector*512, sector*512+size
		if sector < 11 || start >= len(data) || end > len(data) {
			return nil, fmt.Errorf("original data-disk file %q is outside its data sectors", name)
		}
		if size > 0 {
			allocation := extent{start, ((end + 511) / 512) * 512}
			for _, other := range occupied {
				if allocation.start < other.end && allocation.end > other.start {
					return nil, fmt.Errorf("overlapping original data-disk allocation for %q", name)
				}
			}
			occupied = append(occupied, allocation)
		}
		result = append(result, DiskFile{Name: name, Data: bytes.Clone(data[start:end])})
	}
	if !known["data.disk"] {
		return nil, fmt.Errorf("custom replay data disk has no original data.disk marker")
	}
	return result, nil
}

func validDiskName(name string) error {
	_, err := encodeDiskName(name)
	return err
}

func encodeDiskName(name string) ([]byte, error) {
	var encoded []byte
	for _, value := range name {
		if value < 32 || value > 255 || value == '/' || value == '\\' || value == ':' {
			return nil, fmt.Errorf("invalid original data-disk filename %q", name)
		}
		encoded = append(encoded, byte(value))
	}
	if len(encoded) < 1 || len(encoded) > 12 {
		return nil, fmt.Errorf("original data-disk filename must fit twelve Latin-1 bytes")
	}
	return encoded, nil
}

// FilenameKey reproduces 156EA's ASCII-only filename case folding. Accented
// Latin-1 bytes retain their original case even when the host uses Unicode.
func FilenameKey(name string) string {
	return strings.Map(func(value rune) rune {
		if value >= 'A' && value <= 'Z' {
			return value + ('a' - 'A')
		}
		return value
	}, name)
}

// MarshalDataDisk creates a separate, nonbootable original-format data disk.
// It never modifies an existing image. A normal source close records bytes
// actually written, rather than the larger allocation requested at create.
func MarshalDataDisk(files []DiskFile) ([]byte, error) {
	if len(files) > 31 {
		return nil, fmt.Errorf("an original data disk holds 31 files plus data.disk")
	}
	all := make([]DiskFile, 0, len(files)+1)
	all = append(all, DiskFile{Name: "data.disk", Data: []byte("data.disk\x00")})
	all = append(all, files...)
	known := make(map[string]bool)
	data := make([]byte, DataDiskSize)
	sector := 11
	for index, file := range all {
		name, err := encodeDiskName(file.Name)
		if err != nil {
			return nil, err
		}
		key := FilenameKey(file.Name)
		if known[key] {
			return nil, fmt.Errorf("duplicate original data-disk filename %q", file.Name)
		}
		known[key] = true
		if len(file.Data) > 0xfffff || sector >= 1760 || sector*512+len(file.Data) > len(data) {
			return nil, fmt.Errorf("original replay data disk is full")
		}
		record := data[1024+index*16 : 1024+(index+1)*16]
		copy(record[:12], name)
		binary.BigEndian.PutUint32(record[12:], uint32(sector)<<20|uint32(len(file.Data)))
		copy(data[sector*512:], file.Data)
		sector += (len(file.Data) + 511) / 512
	}
	return data, nil
}

// MatchFiles constructs the original empty .rep marker and consecutive .zN
// records. Each copied header receives the chosen file base: 9BCA uses that
// field for Previous/Next. The caller's original clips remain unchanged.
func MatchFiles(base string, clips []*Clip) ([]DiskFile, error) {
	name, err := encodeDiskName(base)
	if err != nil {
		return nil, err
	}
	if len(name) > 8 || strings.Contains(base, ".") {
		return nil, fmt.Errorf("original match replay name must fit eight bytes without an extension")
	}
	if len(clips) > 30 {
		return nil, fmt.Errorf("an original data disk holds at most thirty match replay pieces")
	}
	result := []DiskFile{{Name: base + ".rep"}}
	for index, clip := range clips {
		if clip == nil {
			return nil, fmt.Errorf("missing original replay clip")
		}
		named := *clip
		metadata := named.Metadata()
		metadata.Fields[0] = base
		if err := named.SetMetadata(metadata); err != nil {
			return nil, err
		}
		data, err := named.MarshalBinary()
		if err != nil {
			return nil, err
		}
		result = append(result, DiskFile{Name: base + ".z" + strconv.Itoa(index), Data: data})
	}
	return result, nil
}

// MatchClips follows 9BCA/Previous/Next from the chosen index and each loaded
// header identifier. A missing piece ends the sequence rather than joining a
// disconnected later piece to that original match library.
func MatchClips(files []DiskFile, base string) ([]*Clip, error) {
	byName := make(map[string][]byte, len(files))
	for _, file := range files {
		byName[FilenameKey(file.Name)] = file.Data
	}
	if _, exists := byName[FilenameKey(base)+".rep"]; !exists {
		return nil, fmt.Errorf("original match replay index %q is missing", base+".rep")
	}
	var result []*Clip
	currentBase := base
	for index := 0; index < 31; index++ {
		name := currentBase + ".z" + strconv.Itoa(index)
		data, exists := byName[FilenameKey(name)]
		if !exists {
			break
		}
		clip, err := Parse(data)
		if err != nil {
			return nil, fmt.Errorf("original match piece %s: %w", name, err)
		}
		result = append(result, clip)
		currentBase = clip.Metadata().Fields[0]
	}
	return result, nil
}
