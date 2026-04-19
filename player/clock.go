package player

import (
	"sync"
)

type clock struct {
	sync.RWMutex
	d float64
}

func (c *clock) set(n float64) {
	c.Lock()
	defer c.Unlock()
	c.d = n
}

func (c *clock) get() float64 {
	c.RLock()
	defer c.RUnlock()
	return c.d
}
