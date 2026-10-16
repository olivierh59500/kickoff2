//go:build android

// Package mobile connects the shared game to Android's Ebitengine view.
package mobile

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	enginemobile "github.com/hajimehoshi/ebiten/v2/mobile"

	"github.com/olivierh59500/kickoff2/internal/amigafs"
	"github.com/olivierh59500/kickoff2/internal/game"
	"github.com/olivierh59500/kickoff2/internal/importdata"
	"github.com/olivierh59500/kickoff2/internal/replaydata"
)

// The Android UI thread queues commands; the game thread owns all game state.
type host struct {
	game                  *game.Game
	commands              commandQueue
	configured            bool
	directory             string
	atTitle               atomic.Bool
	importPickerRequested atomic.Bool
	replayExportRequested atomic.Pointer[string]
}

var gameHost = &host{}

func init() {
	// Preserve the Amiga PAL refresh rate on every supported platform.
	ebiten.SetTPS(50)
	ebiten.SetScreenFilterEnabled(false)
	gameHost.atTitle.Store(true)
	enginemobile.SetGame(gameHost)
}

// Configure enables game creation after EbitenView initializes the JNI context.
func Configure(directory string) { gameHost.enqueue(command{kind: configure, directory: directory}) }

// RequestPause preserves an active match after an Android lifecycle transition.
func RequestPause() {
	completed := make(chan struct{})
	gameHost.enqueue(command{kind: pause, completed: completed})
	// Let the game thread save settings before Android suspends its render
	// surface. Bound the wait when the surface has already stopped updating.
	select {
	case <-completed:
	case <-time.After(250 * time.Millisecond):
	}
}

// RequestBack gives the system Back gesture the game's contextual navigation.
func RequestBack() { gameHost.enqueue(command{kind: back}) }

// CancelInput releases unfinished touches after focus or surface changes.
func CancelInput() { gameHost.enqueue(command{kind: cancelInput}) }

// IsAtTitle lets Android close the activity when Back is pressed at the title.
func IsAtTitle() bool { return gameHost.atTitle.Load() }

// TakeImportPickerRequest polls a user-triggered document request on Android's UI thread.
func TakeImportPickerRequest() bool { return gameHost.importPickerRequested.Swap(false) }

// TakeReplayExportRequest transfers an explicit replay or competition export to the Android
// document chooser, keeping all mutable game state on the game thread.
func TakeReplayExportRequest() string {
	if filename := gameHost.replayExportRequested.Swap(nil); filename != nil {
		return *filename
	}
	return ""
}

// ImportCompleted reports a copied document or cancellation to the game thread.
func ImportCompleted(message string) {
	gameHost.enqueue(command{kind: importCompleted, message: message})
}

// ExportCompleted reports a document destination result on the game thread.
func ExportCompleted(message string) {
	gameHost.enqueue(command{kind: exportCompleted, message: message})
}

// ValidateImportedFile checks the private copy before it enters the browser.
// It reads bounded bytes without accessing graphics, audio, or mutable game state.
func ValidateImportedFile(filename string) string {
	kind := strings.ToLower(filepath.Ext(filename))
	if kind == ".sqd" || kind == ".tac" {
		_, _, err := importdata.Read(importdata.File{Path: filename, Kind: kind[1:]})
		if err != nil {
			return err.Error()
		}
		return ""
	}
	if kind == ".rep" || replaydata.IsClipFilename(filename) {
		if _, err := importdata.ReadReplays(importdata.File{Path: filename, Kind: "replay"}); err != nil {
			return err.Error()
		}
		return ""
	}
	if kind != ".adf" {
		return "Select an original squad, tactic, replay or data ADF."
	}
	file, err := os.Open(filename)
	if err != nil {
		return err.Error()
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, 32*1024*1024+1))
	if err != nil {
		return err.Error()
	}
	if len(data) > 32*1024*1024 {
		return "The data disk exceeds 32 MiB."
	}
	if _, err = amigafs.ParseDisk(data); err != nil {
		if _, replayErr := replaydata.ParseDataDisk(data); replayErr != nil {
			return err.Error()
		}
		if _, replayErr := importdata.ReadReplays(importdata.File{Path: filename, Kind: "replay"}); replayErr != nil {
			return replayErr.Error()
		}
	}
	return ""
}

// Dummy retains an exported binding symbol for the Android library generator.
func Dummy() {}

func (h *host) enqueue(request command) {
	h.commands.enqueue(request)
}

func (h *host) Update() error {
	for _, request := range h.commands.drain() {
		if request.kind == configure {
			h.configured = true
			h.directory = request.directory
		}
		if h.game == nil {
			if request.completed != nil {
				close(request.completed)
			}
			continue
		}
		switch request.kind {
		case pause:
			h.game.Pause()
		case back:
			h.game.Back()
		case cancelInput:
			h.game.CancelInput()
		case importCompleted:
			h.game.RefreshImports(request.message)
		case exportCompleted:
			h.game.ReplayExportCompleted(request.message)
		}
		if request.completed != nil {
			close(request.completed)
		}
	}
	if h.game == nil {
		if !h.configured {
			return nil
		}
		// Graphics and audio must be created after Android initializes the view.
		h.game = game.New()
		h.game.SetDataDir(h.directory)
		h.game.SetTouchEnabled(true)
		h.game.SetImportPickerEnabled(true)
	}
	if err := h.game.Update(); err != nil {
		return err
	}
	if h.game.TakeImportPickerRequest() {
		h.importPickerRequested.Store(true)
	}
	if filename := h.game.TakeReplayExportRequest(); filename != "" {
		h.replayExportRequested.Store(&filename)
	}
	h.atTitle.Store(h.game.AtTitle())
	return nil
}

func (h *host) Draw(screen *ebiten.Image) {
	if h.game != nil {
		h.game.Draw(screen)
	}
}

func (h *host) Layout(width, height int) (int, int) {
	if h.game != nil {
		return h.game.Layout(width, height)
	}
	return max(1, width), max(1, height)
}
