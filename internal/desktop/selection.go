package desktop

import (
	"image"

	"github.com/egoist/mygo/ui"
	"lumin/internal/capture"
)

// SelectionView owns only persistent data and a handle, never built elements.
type SelectionView struct {
	bitmap    *ui.Bitmap
	bounds    image.Rectangle
	selection capture.Selection
	dragging  bool
	canvas    ui.Handle
	finish    func(capture.Selection, error)
}

func NewSelection(img image.Image, finish func(capture.Selection, error)) *SelectionView {
	return &SelectionView{bitmap: ui.NewBitmap(img), bounds: img.Bounds(), finish: finish}
}

func fitImage(box ui.Rect, bounds image.Rectangle) ui.Rect {
	scale := min(box.W/float32(bounds.Dx()), box.H/float32(bounds.Dy()))
	w, h := float32(bounds.Dx())*scale, float32(bounds.Dy())*scale
	return ui.Rect{X: box.X + (box.W-w)/2, Y: box.Y + (box.H-h)/2, W: w, H: h}
}

func (s *SelectionView) View(c *ui.Context) {
	c.OnShortcut(0, ui.KeyEscape, func() { s.finish(capture.Selection{}, capture.ErrCanceled) })
	c.OnShortcut(0, ui.KeyEnter, s.confirm)
	ui.Column(c).Fill().Background(ui.Hex("#101319")).Children(func() {
		ui.Row(c).Padding(12, 20).Gap(12).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "拖拽框选区域 · Enter 确认 · Esc 取消").TextColor(ui.Hex("#ffffff")).Grow(1)
			r := s.selection.Pixels(s.bounds)
			ui.Textf(c, "%d × %d px", r.Dx(), r.Dy()).TextColor(ui.Hex("#cccccc"))
			ui.Button(c, "取消截图").OnClick(func() { s.finish(capture.Selection{}, capture.ErrCanceled) })
			ui.PrimaryButton(c, "识别选区").Disabled(r.Dx() < 2 || r.Dy() < 2).OnClick(s.confirm)
		})
		ui.Box(c.Key("selection-canvas")).Label("截图框选画布").Grow(1).Focusable().AutoFocus().Bind(&s.canvas).
			HandleInput(func(ev ui.InputEvent) bool {
				box := s.canvas.Bounds(c)
				box.X, box.Y = 0, 0
				fit := fitImage(box, s.bounds)
				if fit.W <= 0 || fit.H <= 0 {
					return false
				}
				x, y := float64((ev.X-fit.X)/fit.W), float64((ev.Y-fit.Y)/fit.H)
				switch ev.Kind {
				case ui.InputPointerDown:
					if ev.Button != 0 || x < 0 || x > 1 || y < 0 || y > 1 {
						return false
					}
					s.dragging = true
					s.selection = capture.Selection{X0: x, Y0: y, X1: x, Y1: y}
					return true
				case ui.InputPointerMove:
					if !s.dragging {
						return false
					}
					s.selection.X1 = max(0, min(1, x))
					s.selection.Y1 = max(0, min(1, y))
					return true
				case ui.InputPointerUp:
					if !s.dragging || ev.Button != 0 {
						return false
					}
					s.dragging = false
					if ev.Canceled {
						s.selection = capture.Selection{}
					} else {
						s.selection.X1 = max(0, min(1, x))
						s.selection.Y1 = max(0, min(1, y))
					}
					return true
				}
				return false
			}).Draw(func(p *ui.Painter, r ui.Rect) {
			fit := fitImage(r, s.bounds)
			p.Image(s.bitmap, fit, ui.Contain)
			x0, x1 := float32(min(s.selection.X0, s.selection.X1)), float32(max(s.selection.X0, s.selection.X1))
			y0, y1 := float32(min(s.selection.Y0, s.selection.Y1)), float32(max(s.selection.Y0, s.selection.Y1))
			sel := ui.Rect{X: fit.X + x0*fit.W, Y: fit.Y + y0*fit.H, W: (x1 - x0) * fit.W, H: (y1 - y0) * fit.H}
			shade := ui.RGBA(0, 0, 0, 0.48)
			p.Fill(ui.Rect{X: fit.X, Y: fit.Y, W: fit.W, H: sel.Y - fit.Y}, shade, 0)
			p.Fill(ui.Rect{X: fit.X, Y: sel.Y + sel.H, W: fit.W, H: fit.Y + fit.H - sel.Y - sel.H}, shade, 0)
			p.Fill(ui.Rect{X: fit.X, Y: sel.Y, W: sel.X - fit.X, H: sel.H}, shade, 0)
			p.Fill(ui.Rect{X: sel.X + sel.W, Y: sel.Y, W: fit.X + fit.W - sel.X - sel.W, H: sel.H}, shade, 0)
			if sel.W > 0 && sel.H > 0 {
				p.Stroke(sel, ui.Hex("#60a5fa"), 0, 2)
			}
		})
	})
}

func (s *SelectionView) confirm() {
	r := s.selection.Pixels(s.bounds)
	if r.Dx() >= 2 && r.Dy() >= 2 {
		s.finish(s.selection, nil)
	}
}
