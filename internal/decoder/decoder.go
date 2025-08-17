package decoder

import (
	"github.com/asticode/go-astiav"
	"github.com/asticode/go-astikit"
)

// UTILITY
var (
	closer *astikit.Closer = astikit.NewCloser()
)

// AUDIO
var (
	CHANNEL_LAYOUT = astiav.ChannelLayoutStereo
	FORMAT_TYPE    = astiav.SampleFormatFlt
	SAMPLE_RATE    = 44100
	NB_SAMPLES     = 4096
)

func Free() {
	closer.Close()
}
