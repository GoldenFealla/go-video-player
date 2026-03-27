package player

import (
	"fmt"
	"math"
	"time"
)

const (
	syncThreshold = 0.01
)

func VideoSync(clock *Clock, in <-chan VideoFrame, out chan<- VideoFrame) {
	var lastPTS float64
	var lastDelay float64 = 1.0 / 30.0

	for vf := range in {
		pts := vf.PTS
		delay := pts - lastPTS

		if delay <= 0 || delay > 1 {
			delay = lastDelay
		}

		// sync to audio clock
		audio := clock.Audio()
		diff := pts - audio

		fmt.Printf("V %.3f  A %.3f  D %+0.3f\r", pts, audio, diff)

		if math.Abs(diff) < 0.1 {
			delay += diff
		}

		if delay < 0 {
			delay = 0
		}

		time.Sleep(time.Duration(delay * float64(time.Second)))

		lastDelay = delay
		lastPTS = pts

		out <- vf
	}
}
