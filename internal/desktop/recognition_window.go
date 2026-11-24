package desktop

import (
	"context"
	"errors"
	"image"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"lumin/internal/capture"
	"lumin/internal/domain"
)

// Recognition owns its text, image and job independently of the translator.
// Images are immutable after publication; temporary files belong to the worker.
type recognitionState struct {
	text, status string
	image        image.Image
	bitmap       *ui.Bitmap
	busy, pinned bool
	generation   uint64
	cancel       context.CancelFunc
}

func (a *App) showRecognition() {
	if a.ports.ShowRecognizer != nil {
		a.ports.ShowRecognizer()
	}
}

func (a *App) CancelRecognition() {
	r := &a.recognition
	r.generation++
	if r.cancel != nil {
		r.cancel()
		r.cancel = nil
	}
	if r.busy {
		r.status = "已取消识别"
	}
	r.busy = false
}

func (a *App) DismissRecognition() {
	a.CancelRecognition()
	if a.ports.HideRecognizer != nil {
		a.ports.HideRecognizer()
	}
}

// A retained image retries recognition without another file dialog or screenshot.
func (a *App) startRecognition(screenshot bool, retained image.Image) {
	if a.backup.restoring {
		return
	}
	r := &a.recognition
	if r.busy {
		return
	}
	if retained == nil && (screenshot && a.ports.CaptureArea == nil || !screenshot && a.ports.ChooseImage == nil) {
		r.status = "图片获取接口不可用"
		a.showRecognition()
		return
	}
	a.CancelRecognition()
	r.busy, r.status = true, "正在识别…"
	gen := r.generation
	cfg := a.settings.Clone()
	recognizer, err := a.ocrEngine(cfg)
	if err != nil {
		r.busy = false
		r.status = err.Error()
		a.status = err.Error()
		a.openServices("OCR 服务")
		return
	}
	ctx, cancel := context.WithTimeout(a.root, 90*time.Second)
	r.cancel = cancel
	go func() {
		defer cancel()
		img := retained
		var err error
		if img == nil {
			if screenshot {
				img, err = a.ports.CaptureArea(ctx)
			} else {
				var path string
				path, err = a.ports.ChooseImage()
				if err == nil && path != "" {
					img, err = capture.ReadImage(path)
				}
			}
		}
		// Publish preview before calling the service, including when OCR later fails.
		if err == nil && img != nil && ctx.Err() == nil {
			preview := img
			a.post(func() {
				if a.recognition.generation != gen {
					return
				}
				a.recognition.image, a.recognition.bitmap = preview, ui.NewBitmap(preview)
				a.recognition.text = ""
				a.showRecognition()
			})
		}
		var text string
		if err == nil && img != nil && ctx.Err() == nil {
			var path string
			var cleanup func()
			path, cleanup, err = capture.TempPNG(img)
			if cleanup != nil {
				defer cleanup()
			}
			if err == nil {
				ocrDone := capture.TracePhase("ocr")
				text, err = recognizer.Recognize(ctx, path, cfg.OCRLanguages)
				ocrDone()
			}
		}
		if err == nil {
			err = ctx.Err()
		}
		a.post(func() {
			r := &a.recognition
			if r.generation != gen {
				return
			}
			r.busy, r.cancel = false, nil
			switch {
			case errors.Is(err, capture.ErrCanceled), errors.Is(err, context.Canceled):
				r.status = "已取消识别"
			case err != nil:
				r.status = err.Error()
				a.showRecognition()
			case img == nil:
				r.status = "已取消图片选择"
			default:
				r.text, r.status = text, "识别完成，可编辑、复制或转入翻译"
				if strings.TrimSpace(text) == "" {
					r.status = "未识别到文字，可重新识别或更换图片"
				}
				a.showRecognition()
			}
		})
	}()
}

func (a *App) RecognitionView(c *ui.Context) {
	a.applyAppearance(c)
	r, t := &a.recognition, c.Theme()
	c.OnShortcut(0, ui.KeyEscape, a.DismissRecognition)
	ui.Column(c).Fill().Background(t.Background).Padding(14).Gap(10).Children(func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			label := "置顶"
			if r.pinned {
				label = "取消置顶"
			}
			ui.Button(c, label).OnClick(func() {
				r.pinned = !r.pinned
				if a.ports.SetRecognitionPinned != nil {
					a.ports.SetRecognitionPinned(r.pinned)
				}
			})
			ui.Spacer(c)
			ui.Text(c, "文字识别").Bold()
			ui.Spacer(c)
			ui.Button(c, "关闭").OnClick(a.DismissRecognition)
		})
		ui.Box(c.Key("recognition-preview")).Label("识别图片预览").Height(170).Shrink(0).Background(t.Surface).Draw(func(p *ui.Painter, box ui.Rect) {
			if r.bitmap != nil {
				p.Image(r.bitmap, box, ui.Contain)
			}
		})
		ui.Row(c).Gap(8).Children(func() {
			ui.Button(c, "选择图片").Disabled(r.busy).OnClick(a.RecognizeImage)
			ui.Button(c, "截图识字").Disabled(r.busy).OnClick(func() { a.Screenshot(false) })
			ui.Button(c, "重新识别").Disabled(r.busy || r.image == nil).OnClick(func() { a.startRecognition(false, r.image) })
			if r.busy {
				ui.Button(c, "取消").OnClick(a.CancelRecognition)
			}
		})
		ui.TextArea(c.Key("recognition-text"), &r.text).Label("识别结果").Placeholder("识别结果会显示在这里…").MaxLength(domain.MaxTextRunes).Grow(1).MinHeight(90).Disabled(r.busy)
		ui.Row(c).Gap(8).Children(func() {
			ui.Button(c, "复制文本").Disabled(r.busy || strings.TrimSpace(r.text) == "").OnClick(func() {
				if a.ports.WriteClipboard != nil {
					a.ports.WriteClipboard(r.text)
					r.status = "已复制"
				}
			})
			ui.Button(c, "去换行").Disabled(r.busy || r.text == "").OnClick(func() { r.text = strings.Join(strings.Fields(r.text), " ") })
			ui.Spacer(c)
			ui.PrimaryButton(c, "翻译文本").Disabled(r.busy || strings.TrimSpace(r.text) == "").OnClick(func() { a.TranslateText(r.text) })
		})
		ui.Text(c.Key("recognition-status"), r.status).Role(ui.RoleStatus).FontSize(12).TextColor(t.TextMuted).MaxHeight(52)
	})
}
