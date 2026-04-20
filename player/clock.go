package player

import (
	"math"
	"sync/atomic"
)

type clock struct {
	t atomic.Uint64
}

func (c *clock) set(f float64) {
	c.t.Store(math.Float64bits(f))
}

func (c *clock) get() float64 {
	return math.Float64frombits(c.t.Load())
}
