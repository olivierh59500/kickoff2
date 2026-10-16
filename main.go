// Kick Off 2 runs the native Go football engine with Ebitengine presentation.
package main

import (
	"flag"
	"fmt"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/olivierh59500/kickoff2/internal/autoplay"
	"github.com/olivierh59500/kickoff2/internal/game"
	"log"
)

func main() {
	screen := flag.String("screen", "", "open a menu: menu, options, kit, help, practice")
	expert := flag.Bool("autoplay", false, "play through the original joystick with the delayed visual human-style controller")
	reaction := flag.Int("reaction", 6, "visual reaction delay in PAL frames")
	touch := flag.Bool("touch", false, "preview the Android touch controls")
	fullscreen := flag.Bool("fullscreen", false, "start in fullscreen")
	mute := flag.Bool("mute", false, "disable audio")
	match := flag.Bool("match", false, "start a match immediately")
	practice := flag.Bool("practice", false, "start skills practice immediately")
	aftertouch := flag.Bool("aftertouch", false, "enable the original After Touch option")
	demo := flag.Bool("demo", false, "let the computer control both teams")
	smoke := flag.Int("smoke", 0, "run deterministic automatic input for this many updates")
	smokeInput := flag.Bool("smoke-input", true, "supply automatic movement/Fire during a smoke check; false retains real host controls")
	capture := flag.String("capture", "", "write the final smoke frame as a PNG")
	scale := flag.Int("scale", 3, "initial window scale (1 to 8)")
	imports := flag.String("import-dir", "", "directory containing original Player Manager SQD and TAC files")
	data := flag.String("data-dir", "", "override the configuration and save directory")
	originalReplay := flag.String("original-replay", "", "open an original .grp, .zN, .rep or replay data ADF")
	replayEntry := flag.String("replay-entry", "", "original replay filename within the data ADF")
	recordOriginal := flag.String("record-original", "", "create a new original .grp recording after the game exits")
	flag.Parse()
	if *scale < 1 || *scale > 8 || *smoke < 0 {
		log.Fatal("invalid scale or smoke count")
	}
	if *capture != "" && *smoke == 0 {
		log.Fatal("capture requires a positive smoke count")
	}
	if *smoke > 0 {
		// Unattended render checks must finish when another window has focus.
		ebiten.SetRunnableOnUnfocused(true)
	}
	app := game.New()
	if *data != "" {
		app.SetDataDir(*data)
	}
	if *imports != "" {
		app.SetImportDir(*imports)
	}
	app.SetTouchEnabled(*touch)
	if *mute {
		app.SetMuted(true)
	}
	app.SetSmoke(*smoke, *capture)
	app.SetSmokeInput(*smokeInput)
	if *aftertouch {
		app.SetAftertouch(true)
	}
	if *expert {
		options := autoplay.DefaultOptions()
		options.ReactionFrames = *reaction
		app.SetAutoplay(options)
	}
	if *expert || *match || *practice || *demo || *smoke > 0 && *screen == "" {
		app.Start(*practice, *demo)
	}
	if *screen != "" {
		if err := app.ShowScreen(*screen); err != nil {
			log.Fatal(err)
		}
	}
	if *originalReplay != "" {
		if err := app.OpenOriginalReplay(*originalReplay, *replayEntry); err != nil {
			log.Fatal(err)
		}
	}
	w := 320
	if *touch {
		w = 480
	}
	title := "Kick Off 2 — Go / Ebitengine"
	if *smoke > 0 {
		if *smokeInput || *expert {
			title += " [AUTOMATIC INPUT TEST]"
		} else {
			title += " [HOST CONTROL TEST]"
		}
	}
	ebiten.SetWindowTitle(title)
	ebiten.SetWindowSize(w**scale, 256**scale)
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	ebiten.SetFullscreen(*fullscreen)
	ebiten.SetScreenFilterEnabled(false)
	ebiten.SetTPS(50)
	defer app.Close()
	if err := ebiten.RunGame(app); err != nil {
		log.Fatal(err)
	}
	if *smoke > 0 {
		fmt.Println(app.Report())
	}
	if *recordOriginal != "" {
		if err := app.SaveOriginalRecording(*recordOriginal); err != nil {
			log.Fatal(err)
		}
	}
}
