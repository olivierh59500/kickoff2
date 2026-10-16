package importdata

import (
	"fmt"
	"github.com/olivierh59500/kickoff2/internal/amigafs"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

const maximumDiskBytes = 32 * 1024 * 1024

// diskFiles exposes original SQD/TAC files without extracting or modifying the
// disk. The browser uses the same import validation for files and ADF entries.
func diskFiles(filename, kind string) ([]File, error) {
	disk, err := readDisk(filename)
	if err != nil {
		return nil, err
	}
	var result []File
	for _, entry := range disk.Entries() {
		if entry.Directory || entry.Size > 65536 {
			continue
		}
		extension := strings.ToLower(path.Ext(entry.Path))
		if extension != ".sqd" && extension != ".tac" && extension != ".lge" && extension != ".cup" || kind != "" && extension != "."+kind {
			continue
		}
		result = append(result, File{Name: filepath.Base(filename) + " / " + entry.Path, Path: filename, Kind: extension[1:], Size: int64(entry.Size), DiskEntry: entry.Path})
	}
	return result, nil
}
func readDisk(filename string) (*amigafs.Disk, error) {
	info, err := os.Lstat(filename)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > maximumDiskBytes {
		return nil, fmt.Errorf("original data disk is not a bounded regular file")
	}
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maximumDiskBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maximumDiskBytes {
		return nil, fmt.Errorf("original data disk exceeds 32 MiB")
	}
	return amigafs.ParseDisk(data)
}
