package player

import (
	"sync"

	"github.com/asticode/go-astiav"
)

type clock struct {
	sync.RWMutex
	d int64
	b astiav.Rational
}

func (c *clock) set(n int64) {
	c.Lock()
	defer c.Unlock()
	c.d = n
}

func (c *clock) get() int64 {
	c.RLock()
	defer c.RUnlock()
	return c.d
}

func (c *clock) time() float64 {
	c.RLock()
	defer c.RUnlock()
	return c.b.Float64() * float64(c.d)
}
