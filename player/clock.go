package player

import "sync"

type clock struct {
	sync.RWMutex
	t uint32
}

func (c *clock) set(n uint32) {
	c.Lock()
	defer c.Unlock()
	c.t = n
}

func (c *clock) get() uint32 {
	c.RLock()
	defer c.RUnlock()
	return c.t
}
