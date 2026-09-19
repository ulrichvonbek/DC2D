package main

import (
	"errors"
	"log"

	"github.com/hajimehoshi/ebiten/v2"
)

// errQuit is the clean-shutdown sentinel Update returns when the player
// leaves the death screen with ESC; RunGame only reports real failures.
var errQuit = errors.New("player quit the game")

const (
	screenWidth  = 640
	screenHeight = 480
	screenWorldW = 76
	screenWorldH = 48
)

func main() {
	ebiten.SetWindowSize(screenWidth, screenHeight+statusBarHeight+infoPanelHeight)
	ebiten.SetWindowTitle("Dungeon Crawl")

	// Prefer a monsters.yaml on disk (edit-without-rebuild), falling back to
	// the data embedded in the binary.
	applyMonsterOverride()

	game := NewGame()

	if err := ebiten.RunGame(game); err != nil && err != errQuit {
		log.Fatal(err)
	}
}
