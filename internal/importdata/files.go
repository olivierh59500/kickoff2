// Package importdata validates original squad, tactic, competition and replay data without
// executing disk programs or rewriting the user's source files.
package importdata

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/olivierh59500/kickoff2/internal/engine"
)

type File struct {
	Name, Path, Kind string
	Size             int64
	DiskEntry        string
}

// List returns original raw data files from the selected desktop/private folder.
func List(directory, kind string) ([]File, error) {
	if kind == "replay" {
		return listReplays(directory)
	}
	entries, err := os.ReadDir(directory)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var result []File
	for _, entry := range entries {
		if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if ext == ".adf" {
			files, err := diskFiles(filepath.Join(directory, entry.Name()), kind)
			if err != nil {
				return nil, fmt.Errorf("data disk %s: %w", entry.Name(), err)
			}
			result = append(result, files...)
			continue
		}
		if ext != ".sqd" && ext != ".tac" && ext != ".lge" && ext != ".cup" || kind != "" && ext != "."+kind {
			continue
		}
		info, err := entry.Info()
		if err != nil || !info.Mode().IsRegular() || info.Size() > 65536 {
			continue
		}
		result = append(result, File{Name: entry.Name(), Path: filepath.Join(directory, entry.Name()), Kind: ext[1:], Size: info.Size()})
	}
	sort.Slice(result, func(i, j int) bool { return strings.ToLower(result[i].Name) < strings.ToLower(result[j].Name) })
	return result, nil
}

// Read validates the complete bounded file before its bytes can enter a match.
// The checksum identifies the unchanged source resource in import/save reports.
func Read(file File) ([]byte, string, error) {
	maximum := int64(65536)
	if file.Kind == "tournament" {
		maximum = 2 * 1024 * 1024
	}
	if file.DiskEntry != "" {
		disk, err := readDisk(file.Path)
		if err != nil {
			return nil, "", err
		}
		data, err := disk.ReadFile(file.DiskEntry)
		if err != nil {
			return nil, "", err
		}
		if int64(len(data)) > maximum {
			return nil, "", fmt.Errorf("original disk resource exceeds size limit")
		}
		if err = validate(file.Kind, data); err != nil {
			return nil, "", err
		}
		return data, fmt.Sprintf("%x", sha256.Sum256(data)), nil
	}
	info, err := os.Lstat(file.Path)
	if err != nil {
		return nil, "", err
	}
	if !info.Mode().IsRegular() || info.Size() > maximum {
		return nil, "", fmt.Errorf("original data file is not a bounded regular file")
	}
	f, err := os.Open(file.Path)
	if err != nil {
		return nil, "", err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maximum+1))
	if err != nil {
		return nil, "", err
	}
	if int64(len(data)) > maximum {
		return nil, "", fmt.Errorf("original data file exceeds its size limit")
	}
	if err = validate(file.Kind, data); err != nil {
		return nil, "", err
	}

	return data, fmt.Sprintf("%x", sha256.Sum256(data)), nil
}

func validate(kind string, data []byte) error {
	switch kind {
	case "sqd":
		_, err := engine.DecodeSquad(data)
		return err
	case "tac":
		_, err := engine.DecodeTactic(data)
		return err
	case "lge", "cup":
		_, err := engine.DecodeOriginalTournament(data, kind == "cup", [3]uint16{0, 1990, 0})
		return err
	case "tournament":
		_, err := engine.LoadTournament(data)
		return err
	default:
		return fmt.Errorf("unsupported original data-file type %q", kind)
	}
}
