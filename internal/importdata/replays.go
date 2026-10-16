package importdata

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/olivierh59500/kickoff2/internal/replaydata"
)

func readReplayResource(filename string, limit int64) ([]byte, error) {
	info, err := os.Lstat(filename)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > limit {
		return nil, fmt.Errorf("original replay resource is not a bounded regular file")
	}
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil || int64(len(data)) > limit {
		return nil, fmt.Errorf("original replay resource exceeds its size limit")
	}
	return data, nil
}

func listReplays(directory string) ([]File, error) {
	var result []File
	var visit func(string, string, int) error
	visit = func(dir, prefix string, depth int) error {
		entries, err := os.ReadDir(dir)
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if entry.Type()&os.ModeSymlink != 0 {
				continue
			}
			filename := filepath.Join(dir, entry.Name())
			label := prefix + entry.Name()
			if entry.IsDir() {
				if depth < 2 {
					if err := visit(filename, label+" / ", depth+1); err != nil {
						return err
					}
				}
				continue
			}
			extension := strings.ToLower(filepath.Ext(entry.Name()))
			if extension == ".adf" {
				data, err := readReplayResource(filename, replaydata.DataDiskSize)
				if err != nil {
					continue
				}
				files, err := replaydata.ParseDataDisk(data)
				if err != nil {
					continue // Other import kinds can own conventional data ADFs.
				}
				for _, file := range files {
					if replaydata.IsClipFilename(file.Name) || strings.EqualFold(filepath.Ext(file.Name), ".rep") {
						result = append(result, File{Name: label + " / " + file.Name, Path: filename, Kind: "replay", DiskEntry: file.Name, Size: int64(len(file.Data))})
					}
				}
			} else if extension == ".rep" || replaydata.IsClipFilename(entry.Name()) {
				info, err := entry.Info()
				if err == nil && info.Mode().IsRegular() && info.Size() <= 65536 {
					result = append(result, File{Name: label, Path: filename, Kind: "replay", Size: info.Size()})
				}
			}
			if len(result) > 1024 {
				return fmt.Errorf("original replay import folder exceeds1024 entries")
			}
		}
		return nil
	}
	if err := visit(directory, "", 0); err != nil {
		return nil, err
	}
	sort.Slice(result, func(i, j int) bool {
		return replaydata.FilenameKey(result[i].Name) < replaydata.FilenameKey(result[j].Name)
	})
	return result, nil
}

// ReadReplays opens raw movies, a complete match index, or selected data-disk
// entries. Files remain unchanged; callers save independent private copies.
func ReadReplays(file File) ([]*replaydata.Clip, error) {
	var clips []*replaydata.Clip
	if strings.EqualFold(filepath.Ext(file.Path), ".adf") {
		data, err := readReplayResource(file.Path, replaydata.DataDiskSize)
		if err != nil {
			return nil, err
		}
		files, err := replaydata.ParseDataDisk(data)
		if err != nil {
			return nil, err
		}
		if strings.EqualFold(filepath.Ext(file.DiskEntry), ".rep") {
			clips, err = replaydata.MatchClips(files, strings.TrimSuffix(file.DiskEntry, filepath.Ext(file.DiskEntry)))
			if err != nil {
				return nil, err
			}
		} else {
			for _, entry := range files {
				if !replaydata.IsClipFilename(entry.Name) || file.DiskEntry != "" && replaydata.FilenameKey(file.DiskEntry) != replaydata.FilenameKey(entry.Name) {
					continue
				}
				clip, err := replaydata.Parse(entry.Data)
				if err != nil {
					return nil, fmt.Errorf("%s: %w", entry.Name, err)
				}
				clips = append(clips, clip)
			}
		}
	} else if strings.EqualFold(filepath.Ext(file.Path), ".rep") {
		if _, err := readReplayResource(file.Path, 65536); err != nil {
			return nil, err
		}
		entries, err := os.ReadDir(filepath.Dir(file.Path))
		if err != nil {
			return nil, err
		}
		byName := make(map[string]string)
		for _, entry := range entries {
			if !entry.IsDir() && entry.Type()&os.ModeSymlink == 0 && replaydata.IsClipFilename(entry.Name()) {
				key := replaydata.FilenameKey(entry.Name())
				if _, exists := byName[key]; exists {
					return nil, fmt.Errorf("ambiguous original replay filename %s", entry.Name())
				}
				byName[key] = entry.Name()
			}
		}
		base := strings.TrimSuffix(filepath.Base(file.Path), filepath.Ext(file.Path))
		for index := 0; index < 31; index++ {
			name, exists := byName[replaydata.FilenameKey(base)+".z"+strconv.Itoa(index)]
			if !exists {
				break
			}
			data, err := readReplayResource(filepath.Join(filepath.Dir(file.Path), name), 65536)
			if err != nil {
				return nil, err
			}
			clip, err := replaydata.Parse(data)
			if err != nil {
				return nil, err
			}
			clips = append(clips, clip)
			base = clip.Metadata().Fields[0]
		}
	} else {
		data, err := readReplayResource(file.Path, 65536)
		if err != nil {
			return nil, err
		}
		clip, err := replaydata.Parse(data)
		if err != nil {
			return nil, err
		}
		clips = append(clips, clip)
	}
	if len(clips) == 0 {
		return nil, fmt.Errorf("no original clips found; a match index needs its numbered files or data ADF")
	}
	for _, clip := range clips {
		frames, err := clip.Frames()
		if err != nil || len(frames) == 0 {
			return nil, fmt.Errorf("original replay has no playable frames: %v", err)
		}
	}
	return clips, nil
}
