// Package assets embeds the graphics, audio, squads and tactics used by the game.
package assets

import "embed"

// Files contains the bundled game resources.
//
//go:embed original/images/*.png original/raw/sounds original/raw/expsound original/raw/*.tac original/raw/*.sqd original/raw/style*.iff original/raw/kit*.bl1 original/raw/skit*.bl1 original/raw/logo.bl1 original/raw/pdat.* original/raw/fbgraph4.* original/raw/keeper.iff original/raw/ball.iff original/raw/referee.iff original/raw/cornsel*.iff original/raw/fonts.dat original/raw/goal-sprites.bin original/menus.json original/squads.json original/player-surnames.json
var Files embed.FS
