package widget

import (
	"image"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/paint"
	wi "gioui.org/widget"
)

var (
	imageWidget wi.Image
	imageOp     paint.ImageOp
)

func VideoFrame(gtx C, outputVideo chan image.Image) D {
	select {
	case img := <-outputVideo:
		imageOp = paint.NewImageOp(img)
	default:
	}

	gtx.Source.Execute(op.InvalidateCmd{})

	imageWidget.Src = imageOp
	imageWidget.Fit = wi.Contain
	imageWidget.Position = layout.Center

	return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return imageWidget.Layout(gtx)
	})
}
