package main

import (
	"image/color"
	"log"
	"math"
	"math/rand"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text"
	"golang.org/x/image/font/basicfont"
)

const (
	screenWidth    = 1000
	screenHeight   = 700
	klineWidth     = 30
	klineGap       = 8
	klineMaxHeight = 250.0
)

// KLine 单根K线数据
type KLine struct {
	Open  float64
	Close float64
	High  float64
	Low   float64
}

// Game 游戏主结构
type Game struct {
	klines       []KLine
	currentKline KLine
	score        int
	streak       int
	history      []PredictionHistory
	gameStarted  bool
	showResult   bool
	resultText   string
}

type PredictionHistory struct {
	Predicted string
	Actual    string
	Correct   bool
}

var (
	colorWhite   = color.RGBA{255, 255, 255, 255}
	colorYellow  = color.RGBA{255, 200, 50, 255}
	colorGray    = color.RGBA{150, 150, 150, 255}
	colorGreen   = color.RGBA{100, 255, 100, 255}
	colorRed     = color.RGBA{255, 100, 100, 255}
	colorBlue    = color.RGBA{150, 200, 255, 255}
	colorDarkBG  = color.RGBA{20, 25, 40, 255}
	colorUp      = color.RGBA{220, 50, 50, 255}
	colorDown    = color.RGBA{50, 180, 50, 255}
	colorPreview = color.RGBA{255, 255, 100, 200}
)

// NewGame 创建新游戏
func NewGame() *Game {
	g := &Game{
		klines:      make([]KLine, 0),
		gameStarted: false,
	}
	for i := 0; i < 15; i++ {
		g.klines = append(g.klines, generateRandomKLine())
	}
	g.currentKline = generateRandomKLine()
	return g
}

func generateRandomKLine() KLine {
	open := 100.0 + rand.Float64()*50
	var change float64
	if rand.Float64() > 0.5 {
		change = rand.Float64() * 3
	} else {
		change = -rand.Float64() * 3
	}
	close := open + change
	high := math.Max(open, close) + rand.Float64()*1.5
	low := math.Min(open, close) - rand.Float64()*1.5
	return KLine{Open: open, Close: close, High: high, Low: low}
}

func (g *Game) Update() error {
	if inpututil.IsKeyJustPressed(ebiten.KeySpace) && !g.gameStarted {
		g.startGame()
		return nil
	}

	if !g.gameStarted || g.showResult {
		if inpututil.IsKeyJustPressed(ebiten.KeySpace) && g.showResult {
			g.nextRound()
		}
		return nil
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyArrowUp) {
		g.predict("上涨")
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowDown) {
		g.predict("下跌")
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyR) {
		*g = *NewGame()
		return nil
	}

	return nil
}

func (g *Game) startGame() {
	g.gameStarted = true
	g.klines = make([]KLine, 0)
	for i := 0; i < 15; i++ {
		g.klines = append(g.klines, generateRandomKLine())
	}
	g.currentKline = generateRandomKLine()
	g.score = 0
	g.streak = 0
	g.history = make([]PredictionHistory, 0)
}

func (g *Game) nextRound() {
	g.klines = append(g.klines, g.currentKline)
	g.currentKline = generateRandomKLine()
	g.showResult = false
	g.resultText = ""
}

func (g *Game) predict(direction string) {
	g.showResult = true

	var actual string
	if g.currentKline.Close > g.currentKline.Open {
		actual = "上涨"
	} else {
		actual = "下跌"
	}

	correct := direction == actual
	if correct {
		g.score++
		g.streak++
	} else {
		g.streak = 0
	}

	g.history = append(g.history, PredictionHistory{
		Predicted: direction,
		Actual:    actual,
		Correct:   correct,
	})

	if correct {
		g.resultText = "预测正确！" + direction + " 连续连胜: " + itoa(g.streak)
	} else {
		g.resultText = "预测错误！实际" + actual + "，你猜" + direction
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [12]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

func (g *Game) Draw(screen *ebiten.Image) {
	screen.Fill(colorDarkBG)

	if !g.gameStarted {
		g.drawStartScreen(screen)
		return
	}

	g.drawGameScreen(screen)
}

func (g *Game) drawStartScreen(screen *ebiten.Image) {
	x := screenWidth / 2
	y := 150
	face := basicfont.Face7x13

	text.Draw(screen, "K线预测挑战", face, x-80, y, colorYellow)
	y += 40
	text.Draw(screen, "基于K线走势预测下一根K线涨跌", face, x-120, y, colorGray)
	y += 60
	text.Draw(screen, "按 [空格键] 开始游戏", face, x-80, y, colorWhite)
	y += 40
	text.Draw(screen, "预测下一根K线方向：", face, x-90, y, colorGray)
	y += 30
	text.Draw(screen, "  [↑ 上箭头] - 预测上涨", face, x-100, y, colorGreen)
	y += 25
	text.Draw(screen, "  [↓ 下箭头] - 预测下跌", face, x-100, y, colorRed)
}

func (g *Game) drawGameScreen(screen *ebiten.Image) {
	g.drawKLineChart(screen)
	g.drawUI(screen)
	g.drawPredictionUI(screen)

	if g.showResult {
		g.drawResultOverlay(screen)
	}
}

func (g *Game) drawKLineChart(screen *ebiten.Image) {
	var minPrice, maxPrice float64 = 9999, 0
	allKlines := make([]KLine, len(g.klines)+1)
	copy(allKlines, g.klines)
	allKlines[len(g.klines)] = g.currentKline

	for _, k := range allKlines {
		if k.Low < minPrice {
			minPrice = k.Low
		}
		if k.High > maxPrice {
			maxPrice = k.High
		}
	}
	priceRange := maxPrice - minPrice
	if priceRange == 0 {
		priceRange = 1
	}

	startX := float64(screenWidth/2-(len(g.klines)+1)*(klineWidth+klineGap)/2)
	midY := float64(screenHeight / 2)

	for i, k := range g.klines {
		x := startX + float64(i)*(klineWidth+klineGap)
		if x < -klineWidth || x > float64(screenWidth) {
			continue
		}

		color := colorUp
		if k.Close < k.Open {
			color = colorDown
		}

		openY := midY - ((k.Open-minPrice)/priceRange)*klineMaxHeight + klineMaxHeight/2
		closeY := midY - ((k.Close-minPrice)/priceRange)*klineMaxHeight + klineMaxHeight/2
		highY := midY - ((k.High-minPrice)/priceRange)*klineMaxHeight + klineMaxHeight/2
		lowY := midY - ((k.Low-minPrice)/priceRange)*klineMaxHeight + klineMaxHeight/2

		ebitenutil.DrawRect(screen, x+float64(klineWidth/2)-1, highY, 2, lowY-highY, color)

		entityTop := math.Min(openY, closeY)
		entityHeight := math.Abs(openY - closeY)
		if entityHeight < 2 {
			entityHeight = 2
		}
		ebitenutil.DrawRect(screen, x, entityTop, klineWidth, entityHeight, color)
	}

	if g.currentKline.Open > 0 {
		x := startX + float64(len(g.klines))*(klineWidth+klineGap)
		if x > 0 && x < float64(screenWidth) {
			openY := midY - ((g.currentKline.Open-minPrice)/priceRange)*klineMaxHeight + klineMaxHeight/2
			closeY := midY - ((g.currentKline.Close-minPrice)/priceRange)*klineMaxHeight + klineMaxHeight/2
			highY := midY - ((g.currentKline.High-minPrice)/priceRange)*klineMaxHeight + klineMaxHeight/2
			lowY := midY - ((g.currentKline.Low-minPrice)/priceRange)*klineMaxHeight + klineMaxHeight/2

			ebitenutil.DrawRect(screen, x+float64(klineWidth/2)-1, highY, 2, lowY-highY, colorPreview)
			entityTop := math.Min(openY, closeY)
			entityHeight := math.Abs(openY - closeY)
			if entityHeight < 2 {
				entityHeight = 2
			}
			ebitenutil.DrawRect(screen, x, entityTop, klineWidth, entityHeight, colorPreview)

			text.Draw(screen, "预测", basicfont.Face7x13, int(x), int(highY-10), colorPreview)
		}
	}
}

func (g *Game) drawUI(screen *ebiten.Image) {
	x := 20
	y := 20
	face := basicfont.Face7x13

	text.Draw(screen, "得分: "+itoa(g.score), face, x, y, colorGreen)
	y += 25
	text.Draw(screen, "连胜: "+itoa(g.streak), face, x, y, colorYellow)
	y += 40
	text.Draw(screen, "操作说明:", face, x, y, colorGray)
	y += 20
	text.Draw(screen, "[↑] 预测上涨  [↓] 预测下跌", face, x, y, colorBlue)
	y += 20
	text.Draw(screen, "[R] 重新开始", face, x, y, colorBlue)
}

func (g *Game) drawPredictionUI(screen *ebiten.Image) {
	if !g.showResult {
		x := screenWidth / 2
		y := screenHeight - 80
		text.Draw(screen, "请预测下一根K线方向", basicfont.Face7x13, x-80, y, colorWhite)
	}
}

func (g *Game) drawResultOverlay(screen *ebiten.Image) {
	x := screenWidth / 2
	y := screenHeight / 2 - 30
	face := basicfont.Face7x13

	text.Draw(screen, g.resultText, face, x-len(g.resultText)*3, y, colorWhite)
	y += 40
	text.Draw(screen, "按 [空格键] 继续", face, x-60, y, colorGray)

	y += 60
	text.Draw(screen, "预测历史:", face, x-60, y, colorGray)
	y += 25

	start := len(g.history) - 8
	if start < 0 {
		start = 0
	}
	for i := len(g.history) - 1; i >= start; i-- {
		h := g.history[i]
		status := "✓"
		c := colorGreen
		if !h.Correct {
			status = "✗"
			c = colorRed
		}
		line := status + " " + h.Predicted + "→" + h.Actual
		text.Draw(screen, line, face, x-len(line)*2, y, c)
		y += 20
	}
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return screenWidth, screenHeight
}

func main() {
	ebiten.SetWindowSize(screenWidth, screenHeight)
	ebiten.SetWindowTitle("K线预测挑战")
	ebiten.SetWindowResizable(true)

	game := NewGame()
	if err := ebiten.RunGame(game); err != nil {
		log.Fatal(err)
	}
}
