package main

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	_ "image/png"
	"log"
	"math"
	"math/rand"

	"github.com/dmarkham/goNEAT/neat/genetics"
	"github.com/hajimehoshi/ebiten"
	"github.com/hajimehoshi/ebiten/ebitenutil"
	resources "github.com/hajimehoshi/ebiten/examples/resources/images/flappy"
	"golang.org/x/image/font"
)

func init() {
	//rand.Seed(time.Now().UnixNano())
	//rand.Seed(43)
}

var (
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
	pipeIntervalX    = 16
	pipeGapY         = 6
	maxScore         = 50
)

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
	mode    Mode
	players []*Player
	// Camera
	cameraX    int
	cameraY    int
	Population *genetics.Population
	// Pipes
	pipeTileYs    []int
	gameoverCount int
	steps         uint64
}

func NewGame(pop *genetics.Population) *Game {
	g := &Game{Population: pop}
	g.init()
	return g
}

func (g *Game) init() {
	g.mode = ModeGame

	g.players = make([]*Player, len(g.Population.Organisms))
	for i := 0; i < len(g.players); i++ {
		p := &Player{}
		p.x16 = 0
		p.y16 = 100 * 16
		g.players[i] = p
	}
	g.cameraX = -240
	g.cameraY = 0

	if g.pipeTileYs == nil {

		g.pipeTileYs = make([]int, 256)
		for i := range g.pipeTileYs {
			g.pipeTileYs[i] = rand.Intn(6) + 2
		}
	} else {
		panic("in else")
		g.pipeTileYs[0] = rand.Intn(6) + 2
		g.pipeTileYs[1] = rand.Intn(6) + 2
		g.pipeTileYs[3] = rand.Intn(6) + 2
		g.pipeTileYs[10] = rand.Intn(6) + 2
		g.pipeTileYs[20] = rand.Intn(6) + 2

	}
}

func (g *Game) Update() error {
	deadCount := 0
	switch g.mode {
	case ModeGame:
		g.steps++
		g.cameraX += 2
		nextPipeHight, nextPipeDistance := g.nextPipe()
		for i, p := range g.players {

			if p.dead {
				deadCount++
				continue
				//return errors.New("Dead")
			}

			p.x16 += 32
			org := g.Population.Organisms[i]
			netDepth, err := org.Phenotype.MaxDepth()
			if err != nil {
				panic(fmt.Sprintf("Err 1: %+v", err))
			}
			d := float64((p.x16 + (tileSize * nextPipeDistance)) - p.x16)
			//fmt.Println(i, p.x16/tileSize, g.cameraX, p.x16, p.vy16, nextPipeHight, nextPipeDistance, d)
			//org.Phenotype.LoadSensors([]float64{float64(g.cameraX), float64(p.vy16), float64(p.y16), float64(nextPipeHight * tileSize), d})
			org.Phenotype.LoadSensors([]float64{float64(p.vy16), float64(p.y16), float64(nextPipeHight * tileSize), d})
			// Relax net and get output
			success, err := org.Phenotype.Activate()
			if err != nil {
				org.Fitness = 0
				p.dead = true
				fmt.Println(err)
				continue
				//panic(fmt.Sprintf("Err 3: %+v", err))
			}

			if !success {
				// use depth to ensure relaxation
				for relax := 0; relax <= netDepth; relax++ {
					success, err = org.Phenotype.Activate()
					if err != nil {
						org.Fitness = 0
						p.dead = true
						fmt.Println(err)
						continue
						//panic(fmt.Sprintf("Err 2: %+v", err))
					}
				}
			}
			if !success {
				panic("we didn't have success")
			}
			j := org.Phenotype.Outputs[0].Activation
			org.Phenotype.Flush()
			//fmt.Println("HERE: Jump?", j)
			DoJump := false
			if j >= .5 {
				DoJump = true
			}
			if DoJump {
				p.vy16 = -96
			}
			p.y16 += p.vy16

			// Gravity
			p.vy16 += 4
			if p.vy16 > 96 {
				p.vy16 = 96
			}

			//if p.score() > 0 {
			//	fmt.Println("Winning: ", p.score(), p.y16/16, p.x16, nextPipeHight, nextPipeDistance)
			//}
			if g.hit(i) {
				fitness := float64(p.score()) / maxScore
				//fmt.Println("HIT: ", deadCount, fitness, p.y16/16, p.x16, nextPipeHight, nextPipeDistance)
				org.Fitness = fitness
				org.Error = 1 - org.Fitness
				p.dead = true

			} else if p.score() >= maxScore {
				fitness := 1.0
				//fmt.Println(fitness, p.y16/16, p.x16, nextPipeHight, nextPipeDistance)
				org.Fitness = fitness
				org.Error = 0
				org.IsWinner = true
				p.dead = true
			}
		}

		if deadCount == len(g.players) {
			return errors.New("done")

		}
	}
	return nil
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
	//return int(0 - math.Abs(float64(g.y16)))

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

func (g *Game) hit(playerID int) bool {
	if g.mode != ModeGame {
		return false
	}
	const (
		gopherWidth  = 30
		gopherHeight = 60
	)

	//if g.players[playerID].y16 < 0 {
	//	return true
	//}
	w, h := gopherImage.Size()
	x0 := floorDiv(g.players[playerID].x16, 16) + (w-gopherWidth)/2
	y0 := floorDiv(g.players[playerID].y16, 16) + (h-gopherHeight)/2
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

func (g *Game) drawGopher(screen *ebiten.Image) {

	w, h := gopherImage.Size()
	count := 0
	for _, p := range g.players {

		if p.dead {
			//continue
		}
		op := &ebiten.DrawImageOptions{}
		count++
		if count > 5 {
			//break
		}
		op.GeoM.Translate(-float64(w)/2.0, -float64(h)/2.0)
		op.GeoM.Rotate(float64(p.vy16) / 96.0 * math.Pi / 6)
		op.GeoM.Translate(float64(w)/2.0, float64(h)/2.0)
		op.GeoM.Translate(float64(p.x16/16.0)-float64(g.cameraX), float64(p.y16/16.0)-float64(g.cameraY))
		op.Filter = ebiten.FilterLinear
		screen.DrawImage(gopherImage, op)
	}
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
func (g *Game) Draw(screen *ebiten.Image) error {
	//screen.Clear()
	screen.Fill(color.RGBA{0x80, 0xa0, 0xc0, 0xff})
	g.drawTiles(screen)

	g.drawGopher(screen)

	//scoreStr := fmt.Sprintf("%04d", g.player.score())
	//scoreStr := "Yo"
	//text.Draw(screen, scoreStr, arcadeFont, screenWidth-len(scoreStr)*fontSize, fontSize, color.White)
	ebitenutil.DebugPrint(screen, fmt.Sprintf("TPS: %0.2f", ebiten.CurrentTPS()))
	return nil
}
