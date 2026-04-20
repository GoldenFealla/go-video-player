package player

import (
	"sync"
)

type clock struct {
	sync.RWMutex
	t float64
}

func (c *clock) set(t float64) {
	c.Lock()
	defer c.Unlock()
	c.t = t
}

func (c *clock) get() float64 {
	c.RLock()
	defer c.RUnlock()
	return c.t
}
