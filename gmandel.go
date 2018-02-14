package main

import (
	"fmt"
	"log"
	"math/cmplx"
	"os"
	"sync"
	"time"

	"github.com/gotk3/gotk3/gdk"
	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"
)

const (
	KEY_LEFT  uint = 65361
	KEY_UP    uint = 65362
	KEY_RIGHT uint = 65363
	KEY_DOWN  uint = 65364
)

type Drawer struct {
	buf    *gdk.Pixbuf
	pixels []byte
	width  uint
	height uint
	nchan  uint
	stride uint
}

func DrawerFromPixbuf(buf *gdk.Pixbuf) *Drawer {
	d := &Drawer{}
	d.width = uint(buf.GetWidth())
	d.height = uint(buf.GetHeight())
	d.nchan = uint(buf.GetNChannels())
	d.stride = uint(buf.GetRowstride())
	d.pixels = buf.GetPixels()
	return d
}

func (d *Drawer) SetRGB(x, y uint, r, g, b byte) {
	if x < d.width && y < d.height {
		n := y*d.stride + x*d.nchan
		//println("drawing", x, y, n, len(d.pixels))
		d.pixels[n] = r
		d.pixels[n+1] = g
		d.pixels[n+2] = b
	} else {
		//println("skipping", x, y)
	}
}

type MandelState struct {
	X, Y          float64
	Size          float64
	Width, Height int
}

func (m *MandelState) Scale(factor float64) {
	m.Size *= factor
}

func (m *MandelState) Shift(rdx, rdy float64) {
	m.X += m.Size * rdx
	m.Y += m.Size * rdy
}

func CalcMandelx(s *MandelState, d *Drawer) {
	t1 := time.Now()
	xmin := s.X - s.Size
	xmax := s.X + s.Size
	ymin := s.Y - s.Size*float64(s.Height)/float64(s.Width)
	ymax := s.Y + s.Size*float64(s.Height)/float64(s.Width)

	shift := byte(0)
	for py := 0; py < s.Height; py++ {
		y := float64(py)/float64(s.Height)*(ymax-ymin) + ymin
		for px := 0; px < s.Width; px++ {
			x := float64(px)/float64(s.Width)*(xmax-xmin) + xmin
			z := complex(x, y)
			m := mandelbrot(z) / 16 * 16
			if m != 0 {
				m += shift
			}
			if m == 0 {
				d.SetRGB(uint(px), uint(py), 0, 0, 0)
			} else {
				d.SetRGB(uint(px), uint(py), m, (m+85)%255, (m+170)%255)
			}
		}
	}
	fmt.Fprintf(os.Stderr, "%v\n", time.Since(t1))
}

// get the x and y coordinate for the points px,py in 0,0...width,height
func getXY(s *MandelState, px, py int) (x, y float64) {
	x = s.X - s.Size + float64(px)/float64(s.Width)*s.Size*2.0
	yf := float64(s.Height) / float64(s.Width)
	y = s.Y - s.Size*yf + float64(py)/float64(s.Height)*s.Size*2.0*yf
	return
}

func CalcPoint(s *MandelState, d *Drawer, px, py int) byte {
	x, y := getXY(s, px, py)
	z := complex(x, y)
	m := mandelbrot(z) / 16 * 16
	if m == 0 {
		d.SetRGB(uint(px), uint(py), 0, 0, 0)
	} else {
		d.SetRGB(uint(px), uint(py), m, (m+85)%255, (m+170)%255)
	}
	return m
}

func CalcMandelBorder(s *MandelState, d *Drawer, x0, y0, x1, y1 int) (c byte, uniform bool) {
	uniform = true
	c = CalcPoint(s, d, x0, y0)
	for x := x0; x < x1; x++ {
		if CalcPoint(s, d, x, y0) != c {
			uniform = false
		}
		if CalcPoint(s, d, x, y1-1) != c {
			uniform = false
		}
	}
	for y := y0 + 1; y < y1-1; y++ {
		if CalcPoint(s, d, x0, y) != c {
			uniform = false
		}
		if CalcPoint(s, d, x1-1, y) != c {
			uniform = false
		}
	}
	return c, uniform
}

func CalcMandelRect(s *MandelState, d *Drawer, x0, y0, x1, y1 int, level int) {
	w := x1 - x0
	h := y1 - y0
	color, uniform := CalcMandelBorder(s, d, x0, y0, x1, y1)
	//println(x0, y0, x1, y1, color, uniform)
	if uniform {
		for y := y0 + 1; y < y1-1; y++ {
			for x := x0 + 1; x < x1-1; x++ {
				if color == 0 {
					d.SetRGB(uint(x), uint(y), 0, 0, 0)
				} else {
					d.SetRGB(uint(x), uint(y), color, (color+85)%255, (color+170)%255)
				}
			}
		}
	} else {
		if x0 < x1-1 && y0 < y1-1 {
			if level < 10 {
				level++
				xm := x0 + w/2
				ym := y0 + h/2
				CalcMandelRect(s, d, x0+1, y0+1, xm, ym, level)
				CalcMandelRect(s, d, xm, y0+1, x1-1, ym, level)
				CalcMandelRect(s, d, x0+1, ym, xm, y1-1, level)
				CalcMandelRect(s, d, xm, ym, x1-1, y1-1, level)
			}
		}
	}
}

func CalcMandel(s *MandelState, d *Drawer) {
	t1 := time.Now()
	// for py := 0; py < s.Height; py++ {
	// 	for px := 0; px < s.Width; px++ {
	// 		CalcPoint(s, d, px, py)
	// 	}
	// }
	//CalcMandelRect(s, d, 0, 0, s.Width, s.Height, 0)
	n := 4
	dw := s.Width / n
	dh := s.Height / n
	x := 0
	y := 0

	var w sync.WaitGroup

	w.Add(n * n)

	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			go func(i, j int, wg *sync.WaitGroup) {
				CalcMandelRect(s, d, x+i*dw, y+j*dh, x+(i+1)*dw, y+(j+1)*dh, 0)
				wg.Done()
			}(i, j, &w)
		}
	}
	w.Wait()
	fmt.Fprintf(os.Stderr, "%v\n", time.Since(t1))
}

func main() {
	gtk.Init(nil)

	const appID = "com.github.gmandel"
	application, err := gtk.ApplicationNew(appID, glib.APPLICATION_FLAGS_NONE)
	if err != nil {
		log.Fatal("Could not create application.", err)
	}

	application.Connect("activate", func() {
		// Create ApplicationWindow
		win, err := gtk.ApplicationWindowNew(application)
		if err != nil {
			log.Fatal("Could not create application window.", err)
		}
		win.SetDefaultSize(200, 200)

		width, height := 1280, 720

		buf, err := gdk.PixbufNew(gdk.COLORSPACE_RGB, false, 8, width, height)
		if err != nil {
			log.Fatal("Could not create buffer.", err)
		}

		img, err := gtk.ImageNewFromPixbuf(buf)
		if err != nil {
			log.Fatal("Could not create image.", err)
		}

		d := DrawerFromPixbuf(buf)

		m := &MandelState{Width: width, Height: height, X: -0.5, Y: 0, Size: 2.0}
		CalcMandel(m, d)
		win.Window.Connect("key-press-event", func(w *gtk.ApplicationWindow, ev *gdk.Event) {
			keyEvent := &gdk.EventKey{ev}
			// if move, found := keyMap[keyEvent.KeyVal()]; found {
			// 	move()
			// 	win.QueueDraw()
			// }
			kv := keyEvent.KeyVal()
			switch kv {
			case KEY_LEFT:
				m.Shift(-0.1, 0)
				CalcMandel(m, d)
			case KEY_UP:
				m.Shift(0, -0.1)
				CalcMandel(m, d)
			case KEY_RIGHT:
				m.Shift(0.1, 0)
				CalcMandel(m, d)
			case KEY_DOWN:
				m.Shift(0, 0.1)
				CalcMandel(m, d)
			case 43: // +
				m.Scale(0.8)
				CalcMandel(m, d)
			case 45: // -
				m.Scale(1.2)
				CalcMandel(m, d)
			case 70, 102: // F,f
				m.X = -0.5
				m.Y = 0
				m.Size = 2.0
				CalcMandel(m, d)
			case 81, 113: // Q,q
				os.Exit(0)
			default:
				println("kv=", kv)
			}
			img.SetFromPixbuf(buf)
		})

		win.Add(img)
		win.SetTitle("gmandel")
		win.ShowAll()
	})
	application.Run(os.Args)

}

func mandelbrot(z complex128) byte {
	const iterations = 200
	const contrast = 15

	var v complex128
	for n := uint8(0); n < iterations; n++ {
		v = v*v + z
		if cmplx.Abs(v) > 2 {
			val := 255 - contrast*n
			if val >= 0 {
				return byte(val)
			} else {
				return 0
			}
		}
	}
	return 0
}
