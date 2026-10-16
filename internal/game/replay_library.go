package game

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/olivierh59500/kickoff2/internal/replaydata"
)

// replayClip identifies a portable snapshot clip or unchanged original movie.
type replayClip struct {
	ID, Title, Date string
	Teams, Players  [2]string
	Frames          int
	Original        bool `json:"Original,omitempty"`
	Recorded        bool `json:"Recorded,omitempty"`
}

type goldenCompilation struct {
	ID, Title string
	Clips     []string
}

type replayCatalog struct {
	Version int
	Clips   []replayClip
	Golden  []goldenCompilation
}

func (g *Game) libraryDirectory() string { return filepath.Join(g.dataDir, "replay-library") }

func validReplayID(id string) bool {
	if len(id) != 32 {
		return false
	}
	for _, r := range id {
		if !strings.ContainsRune("0123456789abcdef", r) {
			return false
		}
	}
	return true
}

func (g *Game) loadReplayCatalog() error {
	data, err := os.ReadFile(filepath.Join(g.libraryDirectory(), "catalog.json"))
	if os.IsNotExist(err) {
		g.replayCatalog = replayCatalog{Version: 1}
		return nil
	}
	if err != nil {
		return err
	}
	if len(data) > 1024*1024 {
		return fmt.Errorf("replay catalog exceeds its size limit")
	}
	var catalog replayCatalog
	if json.Unmarshal(data, &catalog) != nil || catalog.Version != 1 || len(catalog.Clips) > 256 || len(catalog.Golden) > 128 {
		return fmt.Errorf("invalid replay catalog")
	}
	known := map[string]bool{}
	for _, clip := range catalog.Clips {
		limit := 750
		if clip.Original {
			limit = 4096 // Source24KiB word capacity, independent of native snapshots.
		}
		if !validReplayID(clip.ID) || known[clip.ID] || clip.Frames < 1 || clip.Frames > limit {
			return fmt.Errorf("invalid replay clip entry")
		}
		known[clip.ID] = true
	}
	for _, golden := range catalog.Golden {
		if !validReplayID(golden.ID) || len(golden.Clips) < 1 || len(golden.Clips) > 64 {
			return fmt.Errorf("invalid Golden Goals compilation")
		}
		for _, id := range golden.Clips {
			if !known[id] {
				return fmt.Errorf("Golden Goals references a missing clip")
			}
		}
	}
	g.replayCatalog = catalog
	return nil
}

func writeReplayJSON(path string, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	if err = os.WriteFile(path+".tmp", data, 0600); err != nil {
		return err
	}
	return os.Rename(path+".tmp", path)
}

func (g *Game) saveReplayCatalog() error {
	g.replayCatalog.Version = 1
	return writeReplayJSON(filepath.Join(g.libraryDirectory(), "catalog.json"), g.replayCatalog)
}

func (g *Game) storeReplayClip(fields [6]string) (replayClip, error) {
	if g.replayLength() == 0 {
		return replayClip{}, fmt.Errorf("no replay has been recorded")
	}
	if err := g.loadReplayCatalog(); err != nil {
		return replayClip{}, err
	}
	if len(g.replayCatalog.Clips) >= 256 {
		return replayClip{}, fmt.Errorf("the replay library is full")
	}
	saved, err := g.currentReplayFile()
	if g.replayStoreOriginal && g.originalReplay == nil && g.recordedMovie != nil {
		saved, err = g.originalRecordedFile()
	}
	if err != nil {
		return replayClip{}, err
	}
	if len(saved.Original) != 0 {
		// Store edits belong to the new private copy, preserving the imported
		// movie and every command/padding byte outside its six header fields.
		clip, err := replaydata.Parse(saved.Original)
		if err != nil {
			return replayClip{}, err
		}
		metadata := clip.Metadata()
		metadata.Fields = fields
		if err := clip.SetMetadata(metadata); err != nil {
			return replayClip{}, err
		}
		saved.Original, err = clip.MarshalBinary()
		if err != nil {
			return replayClip{}, err
		}
	}
	count := g.replayLength()
	if len(saved.Original) != 0 {
		movie, err := replaydata.Parse(saved.Original)
		if err != nil {
			return replayClip{}, err
		}
		frames, err := movie.Frames()
		if err != nil {
			return replayClip{}, err
		}
		count = len(frames)
	}
	clip := replayClip{Title: fields[0], Date: fields[1], Teams: [2]string{fields[2], fields[3]}, Players: [2]string{fields[4], fields[5]}, Frames: count, Original: len(saved.Original) != 0, Recorded: len(saved.Recorded) != 0}
	if clip.Title == "" {
		clip.Title = fmt.Sprintf("REPLAY %03d", len(g.replayCatalog.Clips)+1)
	}
	identity, _ := json.Marshal(struct {
		Clip replayClip
		Tick int64
		Time int64
	}{clip, int64(g.ticks), time.Now().UnixNano()})
	clip.ID = fmt.Sprintf("%x", sha256.Sum256(identity))[:32]
	if err := writeReplayJSON(filepath.Join(g.libraryDirectory(), clip.ID+".json"), saved); err != nil {
		return replayClip{}, err
	}
	g.replayCatalog.Clips = append(g.replayCatalog.Clips, clip)
	return clip, g.saveReplayCatalog()
}

func (g *Game) loadLibraryClip(id string) error {
	if !validReplayID(id) {
		return fmt.Errorf("invalid replay clip identity")
	}
	path := filepath.Join(g.libraryDirectory(), id+".json")
	info, err := os.Stat(path)
	if err != nil || info.Size() > 8*1024*1024 {
		return fmt.Errorf("missing or oversized replay clip")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var saved replayFile
	if json.Unmarshal(data, &saved) != nil {
		return fmt.Errorf("invalid portable replay clip")
	}
	return g.installReplayFile(saved)
}

func (g *Game) saveGoldenGoals(title string, ids []string) error {
	if err := g.loadReplayCatalog(); err != nil {
		return err
	}
	if len(ids) < 1 || len(ids) > 64 || len(g.replayCatalog.Golden) >= 128 {
		return fmt.Errorf("select between1 and64 replay clips")
	}
	known := map[string]bool{}
	for _, clip := range g.replayCatalog.Clips {
		known[clip.ID] = true
	}
	for _, id := range ids {
		if !known[id] {
			return fmt.Errorf("the compilation includes an unknown clip")
		}
	}
	if title == "" {
		title = fmt.Sprintf("GOLDEN GOALS %03d", len(g.replayCatalog.Golden)+1)
	}
	identity := fmt.Sprintf("%s:%d", title, time.Now().UnixNano())
	golden := goldenCompilation{fmt.Sprintf("%x", sha256.Sum256([]byte(identity)))[:32], title, append([]string(nil), ids...)}
	g.replayCatalog.Golden = append(g.replayCatalog.Golden, golden)
	return g.saveReplayCatalog()
}

// startReplayPlaylist loads one bounded clip at a time, preserving each match's
// pitch, teams, kit configuration, and clock without concatenating huge frames.
func (g *Game) startReplayPlaylist(ids []string, returnTo screen) error {
	if len(ids) == 0 {
		return fmt.Errorf("no replay clips selected")
	}
	if err := g.loadLibraryClip(ids[0]); err != nil {
		return err
	}
	g.replayPlaylist = append([]string(nil), ids...)
	g.replayPlaylistIndex = 0
	g.replayReturn, g.screen, g.replaySlow = returnTo, replayScreen, false
	g.replayPaused = false
	return nil
}

func (g *Game) advanceReplayPlaylist() bool {
	if len(g.replayPlaylist) == 0 {
		return false
	}
	g.replayPlaylistIndex++
	if g.replayPlaylistIndex >= len(g.replayPlaylist) {
		g.replayPlaylist = nil
		return false
	}
	if err := g.loadLibraryClip(g.replayPlaylist[g.replayPlaylistIndex]); err != nil {
		g.notice = err.Error()
		g.replayPlaylist = nil
		return false
	}
	return true
}

func (g *Game) deleteLibraryClip(id string) error {
	for _, golden := range g.replayCatalog.Golden {
		for _, used := range golden.Clips {
			if used == id {
				return fmt.Errorf("clip belongs to a Golden Goals compilation")
			}
		}
	}
	for i, clip := range g.replayCatalog.Clips {
		if clip.ID == id {
			g.replayCatalog.Clips = append(g.replayCatalog.Clips[:i], g.replayCatalog.Clips[i+1:]...)
			if err := g.saveReplayCatalog(); err != nil {
				return err
			}
			return os.Remove(filepath.Join(g.libraryDirectory(), id+".json"))
		}
	}
	return fmt.Errorf("unknown replay clip")
}
