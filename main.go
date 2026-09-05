package main

import (
	"log"

	"github.com/hajimehoshi/ebiten/v2"
)

const (
	screenWidth  = 640
	screenHeight = 480
	screenWorldW = 64
	screenWorldH = 48
)

func main() {
	ebiten.SetWindowSize(screenWidth, screenHeight+statusBarHeight)
	ebiten.SetWindowTitle("Dungeon Crawl")

	game := NewGame()

	if err := ebiten.RunGame(game); err != nil {
		log.Fatal(err)
	}
}
