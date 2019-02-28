package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	_ "image/png"
	"log"
	"math"
	"math/rand"
	"os"
	"runtime"
	"time"

	"github.com/golang/freetype/truetype"
	"github.com/hajimehoshi/ebiten"
	"github.com/hajimehoshi/ebiten/audio"
	"github.com/hajimehoshi/ebiten/audio/vorbis"
	"github.com/hajimehoshi/ebiten/audio/wav"
	"github.com/hajimehoshi/ebiten/ebitenutil"
	raudio "github.com/hajimehoshi/ebiten/examples/resources/audio"
	"github.com/hajimehoshi/ebiten/examples/resources/fonts"
	resources "github.com/hajimehoshi/ebiten/examples/resources/images/flappy"
	"github.com/hajimehoshi/ebiten/inpututil"
	"github.com/hajimehoshi/ebiten/text"
	"github.com/yaricom/goNEAT/neat/genetics"
	"golang.org/x/image/font"
)

func init() {
	rand.Seed(time.Now().UnixNano())
	//rand.Seed(43)
}

func floorDiv(x, y int) int {
	d := x / y
	if d*y == x || x >= 0 {
		return d
	}
	return d - 1
}

func floorMod(x, y int) int {
	return x - floorDiv(x, y)*y
}

const (
	screenWidth  = 640
	screenHeight = 480
	//screenWidth      = 800
	//screenHeight     = 600
	tileSize         = 32
	fontSize         = 32
	smallFontSize    = fontSize / 2
	pipeWidth        = tileSize * 2
	pipeStartOffsetX = 8
	pipeGapY         = 6
	//pipeIntervalX    = 16
)

var (
	pipeIntervalX   = 16
	gopherImage     *ebiten.Image
	tilesImage      *ebiten.Image
	arcadeFont      font.Face
	smallArcadeFont font.Face
)

func init() {
	img, _, err := image.Decode(bytes.NewReader(resources.Gopher_png))
	if err != nil {
		log.Fatal(err)
	}
	gopherImage, _ = ebiten.NewImageFromImage(img, ebiten.FilterDefault)

	img, _, err = image.Decode(bytes.NewReader(resources.Tiles_png))
	if err != nil {
		log.Fatal(err)
	}
	tilesImage, _ = ebiten.NewImageFromImage(img, ebiten.FilterDefault)
}

func init() {
	tt, err := truetype.Parse(fonts.ArcadeN_ttf)
	if err != nil {
		log.Fatal(err)
	}
	const dpi = 72
	arcadeFont = truetype.NewFace(tt, &truetype.Options{
		Size:    fontSize,
		DPI:     dpi,
		Hinting: font.HintingFull,
	})
	smallArcadeFont = truetype.NewFace(tt, &truetype.Options{
		Size:    smallFontSize,
		DPI:     dpi,
		Hinting: font.HintingFull,
	})
}

var (
	audioContext *audio.Context
	jumpPlayer   *audio.Player
	hitPlayer    *audio.Player
)

func init() {
	audioContext, _ = audio.NewContext(44100)

	jumpD, err := vorbis.Decode(audioContext, audio.BytesReadSeekCloser(raudio.Jump_ogg))
	if err != nil {
		log.Fatal(err)
	}
	jumpPlayer, err = audio.NewPlayer(audioContext, jumpD)
	if err != nil {
		log.Fatal(err)
	}

	jabD, err := wav.Decode(audioContext, audio.BytesReadSeekCloser(raudio.Jab_wav))
	if err != nil {
		log.Fatal(err)
	}
	hitPlayer, err = audio.NewPlayer(audioContext, jabD)
	if err != nil {
		log.Fatal(err)
	}
}

type Mode int

const (
	ModeTitle Mode = iota
	ModeGame
	ModeGameOver
)

type Player struct {
	// The gopher's position
	x16  int
	y16  int
	vy16 int
	dead bool
}

type Game struct {
	mode   Mode
	player *Player
	genome *genetics.Genome
	// Camera
	cameraX int
	cameraY int

	// Pipes
	pipeTileYs    []int
	gameoverCount int
}

func NewGame() *Game {
	g := &Game{}
	g.init()
	return g
}

func (g *Game) init() {
	if g.genome == nil {
		//fh, err := os.Open("/tmp/out/1/best_0.87000000_24-121")
		//fh, err := os.Open("/tmp/out/1/best_0.42000000_22-97")
		fh, err := os.Open("winner-6.genome")
		//fh, err := os.Open("winner-5.genome")
		//fh, err := os.Open("/tmp/out/1/winner_1.0_7-9")
		if err != nil {
			log.Fatal(err)
		}
		start_genome, err := genetics.ReadGenome(fh, 1)
		if err != nil {
			log.Fatal("Failed to read start genome: ", err)
		}
		//fmt.Println(start_genome)
		start_genome.Genesis(1)

		g.genome = start_genome
	}
	g.mode = ModeGame
	p := &Player{}
	p.x16 = 0
	p.y16 = 100 * 16
	g.player = p
	g.cameraX = -240
	g.cameraY = 0

	g.pipeTileYs = make([]int, 256)
	for i := range g.pipeTileYs {
		g.pipeTileYs[i] = rand.Intn(6) + 2
	}
	//pipeIntervalX = rand.Intn(11) + 5
	//g.pipeTileYs[0] = 2
	//g.pipeTileYs[1] = 2
	//g.pipeTileYs[2] = 4
	//}
	//fmt.Print(g.pipeTileYs)
	//os.Exit(0)
}

func jump() bool {
	if inpututil.IsKeyJustPressed(ebiten.KeySpace) {
		return true
	}
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		return true
	}
	if len(inpututil.JustPressedTouchIDs()) > 0 {
		return true
	}
	return false
}

func (g *Game) Update(screen *ebiten.Image) error {
	switch g.mode {
	case ModeTitle:
		if jump() {
			g.mode = ModeGame
		}
	case ModeGame:
		g.cameraX += 2
		deadCount := 0
		nextPipeHight, nextPipeDistance := g.nextPipe()

		p := g.player
		if p.dead {
			deadCount++
		}
		p.x16 += 32
		//p.x16 += 64
		org := g.genome
		//fmt.Println(p.y16/16, p.x16, nextPipeHight, nextPipeDistance)
		//org.Phenotype.LoadSensors([]float64{float64(p.vy16), float64(p.y16) / 16, float64(nextPipeHight), float64(nextPipeDistance)})
		d := float64((p.x16 + (tileSize * nextPipeDistance)) - p.x16)
		org.Phenotype.LoadSensors([]float64{float64(p.vy16), float64(p.y16), float64(nextPipeHight * tileSize), d})
		netDepth, err := org.Phenotype.MaxDepth()
		if err != nil {
			panic(fmt.Sprintf("Err 1: %+v", err))
		}
		// Relax net and get output
		success, err := org.Phenotype.Activate()
		if err != nil {
			panic(fmt.Sprintf("Err 3: %+v", err))
		}

		if !success {
			// use depth to ensure relaxation
			for relax := 0; relax <= netDepth; relax++ {
				success, err = org.Phenotype.Activate()
				if err != nil {
					panic(fmt.Sprintf("Err 2: %+v", err))
				}
			}
		}
		if !success {
			panic("we didnt have success")
		}
		j := org.Phenotype.Outputs[0].Activation
		//fmt.Println("HERE: ", i, j)
		org.Phenotype.Flush()
		DoJump := false
		if j >= .5 {
			DoJump = true
		}
		if DoJump {

			p.vy16 = -96
			//jumpPlayer.Rewind()
			//jumpPlayer.Play()
		}
		p.y16 += p.vy16

		// Gravity
		p.vy16 += 4
		if p.vy16 > 96 {
			p.vy16 = 96
		}

		if g.hit() {
			//hitPlayer.Rewind()
			//hitPlayer.Play()
			p.dead = true
		}

		if deadCount == 1 {
			fmt.Println("Score: ", g.player.score())
			g.init()
		}
	case ModeGameOver:
		if g.gameoverCount > 0 {
			g.gameoverCount--
		}
		if g.gameoverCount == 0 && jump() {
			g.init()
			g.mode = ModeTitle
		}
	}

	if ebiten.IsDrawingSkipped() {
		return nil
	}
	return g.draw(screen)
}

func (g *Game) nextPipe() (int, int) {

	const (
		nx           = screenWidth / tileSize
		ny           = screenHeight / tileSize
		pipeTileSrcX = 128
		pipeTileSrcY = 192
	)

	for i := -2; i < nx+1; i++ {
		// pipe
		if tileY, ok := g.pipeAt(floorDiv(g.cameraX, tileSize) + i); ok {
			return int(tileY), i
		}
	}
	return g.pipeTileYs[0], 20
}

func (g *Game) draw(screen *ebiten.Image) error {
	screen.Fill(color.RGBA{0x80, 0xa0, 0xc0, 0xff})
	g.drawTiles(screen)
	if g.mode != ModeTitle {
		g.drawGopher(screen)
	}
	var texts []string
	switch g.mode {
	case ModeTitle:
		texts = []string{"FLAPPY GOPHER", "", "", "", "", "PRESS SPACE KEY", "", "OR TOUCH SCREEN"}
	case ModeGameOver:
		texts = []string{"", "GAMEOVER!"}
	}
	for i, l := range texts {
		x := (screenWidth - len(l)*fontSize) / 2
		text.Draw(screen, l, arcadeFont, x, (i+4)*fontSize, color.White)
	}

	if g.mode == ModeTitle {
		msg := []string{
			"Go Gopher by Renee French is",
			"licenced under CC BY 3.0.",
		}
		for i, l := range msg {
			x := (screenWidth - len(l)*smallFontSize) / 2
			text.Draw(screen, l, smallArcadeFont, x, screenHeight-4+(i-1)*smallFontSize, color.White)
		}
	}

	scoreStr := fmt.Sprintf("%04d", g.player.score())

	text.Draw(screen, scoreStr, arcadeFont, screenWidth-len(scoreStr)*fontSize, fontSize, color.White)
	ebitenutil.DebugPrint(screen, fmt.Sprintf("TPS: %0.2f", ebiten.CurrentTPS()))
	return nil
}

func (g *Game) pipeAt(tileX int) (tileY int, ok bool) {
	if (tileX - pipeStartOffsetX) <= 0 {
		return 0, false
	}
	if floorMod(tileX-pipeStartOffsetX, pipeIntervalX) != 0 {
		return 0, false
	}
	idx := floorDiv(tileX-pipeStartOffsetX, pipeIntervalX)
	return g.pipeTileYs[idx%len(g.pipeTileYs)], true
}

func (g *Player) score() int {

	//return g.x16
	x := floorDiv(g.x16, 16) / tileSize
	if (x - pipeStartOffsetX) <= 0 {
		return 0
	}
	s := floorDiv(x-pipeStartOffsetX, pipeIntervalX)
	if s > 0 {
		return s
	}
	return s
	//return int(0 - math.Abs(float64(g.y16)))
}

func (g *Game) hit() bool {
	if g.mode != ModeGame {
		return false
	}
	const (
		gopherWidth  = 30
		gopherHeight = 60
	)

	//if g.player.y16 < 0 {
	//	return true
	//}
	w, h := gopherImage.Size()
	x0 := floorDiv(g.player.x16, 16) + (w-gopherWidth)/2
	y0 := floorDiv(g.player.y16, 16) + (h-gopherHeight)/2
	x1 := x0 + gopherWidth
	y1 := y0 + gopherHeight
	if y0 < -tileSize*4 {
		return true
	}
	if y1 >= screenHeight-tileSize {
		return true
	}
	xMin := floorDiv(x0-pipeWidth, tileSize)
	xMax := floorDiv(x0+gopherWidth, tileSize)
	for x := xMin; x <= xMax; x++ {
		y, ok := g.pipeAt(x)
		if !ok {
			continue
		}
		if x0 >= x*tileSize+pipeWidth {
			continue
		}
		if x1 < x*tileSize {
			continue
		}
		if y0 < y*tileSize {
			return true
		}
		if y1 >= (y+pipeGapY)*tileSize {
			return true
		}
	}
	return false
}

func (g *Game) drawTiles(screen *ebiten.Image) {
	const (
		nx           = screenWidth / tileSize
		ny           = screenHeight / tileSize
		pipeTileSrcX = 128
		pipeTileSrcY = 192
	)

	op := &ebiten.DrawImageOptions{}
	for i := -2; i < nx+1; i++ {
		// ground
		op.GeoM.Reset()
		op.GeoM.Translate(float64(i*tileSize-floorMod(g.cameraX, tileSize)),
			float64((ny-1)*tileSize-floorMod(g.cameraY, tileSize)))
		screen.DrawImage(tilesImage.SubImage(image.Rect(0, 0, tileSize, tileSize)).(*ebiten.Image), op)

		// pipe
		if tileY, ok := g.pipeAt(floorDiv(g.cameraX, tileSize) + i); ok {
			for j := 0; j < tileY; j++ {
				op.GeoM.Reset()
				op.GeoM.Scale(1, -1)
				op.GeoM.Translate(float64(i*tileSize-floorMod(g.cameraX, tileSize)),
					float64(j*tileSize-floorMod(g.cameraY, tileSize)))
				op.GeoM.Translate(0, tileSize)
				var r image.Rectangle
				if j == tileY-1 {
					r = image.Rect(pipeTileSrcX, pipeTileSrcY, pipeTileSrcX+tileSize*2, pipeTileSrcY+tileSize)
				} else {
					r = image.Rect(pipeTileSrcX, pipeTileSrcY+tileSize, pipeTileSrcX+tileSize*2, pipeTileSrcY+tileSize*2)
				}
				screen.DrawImage(tilesImage.SubImage(r).(*ebiten.Image), op)
			}
			for j := tileY + pipeGapY; j < screenHeight/tileSize-1; j++ {
				op.GeoM.Reset()
				op.GeoM.Translate(float64(i*tileSize-floorMod(g.cameraX, tileSize)),
					float64(j*tileSize-floorMod(g.cameraY, tileSize)))
				var r image.Rectangle
				if j == tileY+pipeGapY {
					r = image.Rect(pipeTileSrcX, pipeTileSrcY, pipeTileSrcX+pipeWidth, pipeTileSrcY+tileSize)
				} else {
					r = image.Rect(pipeTileSrcX, pipeTileSrcY+tileSize, pipeTileSrcX+pipeWidth, pipeTileSrcY+tileSize+tileSize)
				}
				screen.DrawImage(tilesImage.SubImage(r).(*ebiten.Image), op)
			}
		}
	}
}

func (g *Game) drawGopher(screen *ebiten.Image) {
	p := g.player
	op := &ebiten.DrawImageOptions{}
	w, h := gopherImage.Size()
	op.GeoM.Translate(-float64(w)/2.0, -float64(h)/2.0)
	op.GeoM.Rotate(float64(p.vy16) / 96.0 * math.Pi / 6)
	op.GeoM.Translate(float64(w)/2.0, float64(h)/2.0)
	op.GeoM.Translate(float64(p.x16/16.0)-float64(g.cameraX), float64(p.y16/16.0)-float64(g.cameraY))
	op.Filter = ebiten.FilterLinear
	screen.DrawImage(gopherImage, op)
}

func main() {
	g := NewGame()

	// On browsers, let's use fullscreen so that this is playable on any browsers.
	// It is planned to ignore the given 'scale' apply fullscreen automatically on browsers (#571).
	if runtime.GOARCH == "js" || runtime.GOOS == "js" {
		ebiten.SetFullscreen(true)
	}
	ebiten.SetRunnableInBackground(true)
	if err := ebiten.Run(g.Update, screenWidth, screenHeight, 1, "Flappy Gopher (Ebiten Demo)"); err != nil {
		panic(err)
	}
}
