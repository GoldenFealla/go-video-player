package widget

import (
	"image"

	"gioui.org/layout"
)

func Controller(gtx C) D {
	return layout.Dimensions{
		Size: image.Pt(0, 0),
	}
}
