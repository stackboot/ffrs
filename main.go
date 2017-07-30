package main

import (
	"bytes"
	"crypto/rand"
	"image"
	"image/color"
	_ "image/png"
	"io"
	"io/ioutil"
	"log"
	"math"
	"net/http"

	"github.com/hajimehoshi/ebiten"
	"github.com/hajimehoshi/ebiten/audio"
	"github.com/hajimehoshi/ebiten/audio/wav"
	"github.com/hajimehoshi/ebiten/ebitenutil"
)

const host = `https://stackboot.github.io/ffrs/`

type rsc struct {
	io.ReadSeeker
}

func (rsc) Close() error {
	return nil
}

func noise(actx *audio.Context, name string) *audio.Player {
	res, err := http.Get(host + name)
	if err != nil {
		panic(err)
	}
	defer res.Body.Close()
	b, err := ioutil.ReadAll(res.Body)
	if err != nil {
		panic(err)
	}
	str, err := wav.Decode(actx, rsc{bytes.NewReader(b)})
	if err != nil {
		panic(err)
	}
	buf := new(bytes.Buffer)
	io.Copy(buf, str)
	p, err := audio.NewPlayerFromBytes(actx, buf.Bytes())
	if err != nil {
		panic(err)
	}
	return p
}

var actx = func() *audio.Context {
	c, err := audio.NewContext(44100)
	if err != nil {
		panic(err)
	}
	return c
}()

var GoodHit = noise(actx, "goodhit.wav")
var BadHit = noise(actx, "badhit.wav")
var Explode = noise(actx, "explode1.wav")

func load(name string) *ebiten.Image {
	res, err := http.Get(host + name)
	if err != nil {
		panic(err)
	}
	defer res.Body.Close()
	i, _, err := image.Decode(res.Body)
	if err != nil {
		panic(err)
	}
	img, err := ebiten.NewImageFromImage(i, ebiten.FilterNearest)
	if err != nil {
		panic(err)
	}
	return img
}

var FuelPng = load("fuel.png")

var NeutronPng = load("neutron.png")

var RodPng = load("rod.png")

var BackgroundPng = load("background.png")

var IntroPng = load("intro.png")

const Width = 320

const Height = 240

var Frames = uint32(0)

type Func func(*ebiten.Image) error

func (f Func) Update(screen *ebiten.Image) error {
	return f(screen)
}

type Interface interface {
	Update(screen *ebiten.Image) error
}

type Sprite interface {
	Interface
	Img() *ebiten.Image
	Opts() *ebiten.DrawImageOptions
	Loc() (x, y float64)
}

type base struct {
	*ebiten.Image
	*ebiten.DrawImageOptions
	X, Y float64
	Func
}

func (b *base) Img() *ebiten.Image {
	return b.Image
}

func (b *base) Opts() *ebiten.DrawImageOptions {
	if b.DrawImageOptions == nil {
		b.DrawImageOptions = new(ebiten.DrawImageOptions)
	}
	b.DrawImageOptions.GeoM.Reset()
	return b.DrawImageOptions
}

func (b *base) Loc() (x, y float64) {
	return b.X, b.Y
}

func (b *base) Update(screen *ebiten.Image) error {
	if b.Func != nil {
		return b.Func(screen)
	}
	return nil
}

type neutron struct {
	base
}

func (n *neutron) LR(*ebiten.Image) error {
	n.X++
	return nil
}

func (n *neutron) RL(*ebiten.Image) error {
	n.X--
	return nil
}

func (n *neutron) Update(screen *ebiten.Image) (err error) {
	return n.base.Update(screen)
}

type neutrons []*neutron

func (n *neutrons) Update(screen *ebiten.Image) (err error) {
	if Frames == 0 {
		neu := &neutron{base: base{Image: NeutronPng, X: 48, Y: 32}}
		neu.Func = neu.LR
		(*n)[0] = neu
		return nil
	}
	if Frames%600 == 0 {
		*n = append(*n, nil)
	}
	for i, s := range *n {
		if s == nil {
			if Frames%60 != 0 {
				continue
			}
			h := (32 * (i + 1)) % Height
			s = &neutron{base: base{Image: NeutronPng, X: Width - 48, Y: float64(h)}}
			s.Func = s.RL
			if i%2 == 0 {
				s.X = 48
				s.Func = s.LR
			}
			(*n)[i] = s
		}
		if err = s.Update(screen); err != nil {
			return err
		}
		if s.Bounds().Add(image.Point{int(s.X), int(s.Y)}).Intersect(ControlRod.Bounds().Add(image.Point{int(ControlRod.X), int(ControlRod.Y)})) != image.ZR {
			(*n)[i] = nil
			GoodHit.Rewind()
			GoodHit.Play()
			continue
		}
		switch {
		case s.X > Width-48:
			RightFuel.Hits++
			(*n)[i], s = nil, nil
		case s.X < 48:
			LeftFuel.Hits++
			(*n)[i], s = nil, nil
		}
		if s == nil {
			BadHit.Rewind()
			BadHit.Play()
			continue
		}
		if err = Update(screen, s); err != nil {
			return err
		}
	}
	/*
		for i, n := range b.neutrons {
			if n == nil {
				n = &neutron{base: base{Image: NeutronPng, X: 32, Y: Height / 2}}
				switch i%2 == 0 {
				case true:
					n.Func = n.LR
				default:
					n.X = Width - 48
					n.Func = n.RL
				}
				b.neutrons[i] = n
				continue
			}
			if n.X > Width-48 || n.X < 48 {
				b.neutrons[i] = nil

			}
	*/
	return nil
}

var Neutrons = make(neutrons, 1, 16)

type controlrod struct {
	base
}

var ControlRod = &controlrod{base: base{Image: RodPng, X: -Width, Y: -Height}}

func (c *controlrod) drop(*ebiten.Image) error {
	c.Y += 2
	if c.Y > Height+32 {
		c.Func = nil
		c.X, c.Y = -Width, -Height
	}
	return nil
}

func (c *controlrod) Update(screen *ebiten.Image) error {
	if ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) && c.Y == -Height {
		x, _ := ebiten.CursorPosition()
		c.X, c.Y = float64(x), -32
		c.Func = c.drop
	}
	return c.base.Update(screen)
}

type fuel struct {
	base
	Hits int
}

func (f *fuel) Update(screen *ebiten.Image) error {
	if f.Hits > 9 {
		meltdown()
		return nil
	}
	f.Opts().ColorM.RotateHue(math.Pi * (float64(f.Hits) * .05))
	return f.base.Update(screen)
}

var LeftFuel = &fuel{base: base{Image: FuelPng, X: 16, Y: Height / 2}}

var RightFuel = &fuel{base: base{Image: FuelPng, X: Width - 16, Y: Height / 2}}

type background struct {
	base
	neutrons [16]*neutron
}

type game []Interface

func Update(screen *ebiten.Image, i Sprite) (err error) {
	if i == nil {
		return nil
	}
	img := i.Img()
	if img == nil {
		return nil
	}
	opts := i.Opts()
	bounds := i.Img().Bounds()
	dx, dy := float64(bounds.Dx()), float64(bounds.Dy())
	opts.GeoM.Translate(dx/-2, dy/-2)
	opts.GeoM.Translate(i.Loc())
	return screen.DrawImage(img, opts)
}

func (g *game) Update(screen *ebiten.Image) (err error) {
	for _, i := range *g {
		if i == nil {
			continue
		}
		if err = i.Update(screen); err != nil {
			return err
		}
		if i, ok := i.(Sprite); ok {
			if err = Update(screen, i); err != nil {
				return err
			}
		}
	}
	Frames++
	return nil
}

var Game game

func intro() {
	Frames = 0
	s := &base{Image: IntroPng, X: Width / 2, Y: Height * 2}
	s.Func = func(screen *ebiten.Image) error {
		if ebiten.IsKeyPressed(ebiten.KeySpace) {
			reset()
			return nil
		}
		if Frames%3 == 0 {
			s.Y--
		}
		if s.Y < -Height {
			s.Y = Height * 2
		}
		return ebitenutil.DebugPrint(screen, "Press space bar to begin simulation.")
	}
	Game = game{s}
}

func meltdown() {
	Explode.Rewind()
	Explode.Play()
	Frames = 0
	str := `You have failed.
And this is what it looks like when
a fission reactor melts down.

Ask anybody. I don't care.
`
	col := color.NRGBA{255, 255, 255, 255}
	buf := make([]byte, 3)
	s := Func(func(screen *ebiten.Image) error {
		rand.Read(buf)
		col.R, col.G, col.B = buf[0], buf[1], buf[2]
		return screen.Fill(col)
	})
	f := Func(func(screen *ebiten.Image) error {
		if Frames == 60*5 {
			str += "\nClick mouse to try again."
		}
		if Frames > 60*5 {
			if ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
				intro()
				return nil
			}
		}
		return ebitenutil.DebugPrint(screen, str)
	})
	Game = game{s, f}
}

func reset() {
	Frames = 0
	Neutrons = Neutrons[:1]
	ControlRod.X, ControlRod.Y, ControlRod.Func = -Width, -Height, nil
	LeftFuel.Hits, RightFuel.Hits = 0, 0
	Game = game{
		&background{base: base{Image: BackgroundPng, X: Width / 2, Y: Height / 2}},
		ControlRod,
		LeftFuel,
		RightFuel,
		&Neutrons,
	}
}

func main() {
	intro()
	log.Fatal(ebiten.Run(Game.Update, Width, Height, 2.0, "Fast Fission Reactor Simulator"))
}

