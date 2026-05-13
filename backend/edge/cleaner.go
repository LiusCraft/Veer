package edge

import (
	"sync"
	"time"
)

type Cleaner struct {
	index    *CacheIndex
	interval time.Duration
	stopChan chan struct{}
	wg       sync.WaitGroup
}

func NewCleaner(index *CacheIndex, interval time.Duration) *Cleaner {
	return &Cleaner{
		index:    index,
		interval: interval,
		stopChan: make(chan struct{}),
	}
}

func (c *Cleaner) Start() {
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		ticker := time.NewTicker(c.interval)
		defer ticker.Stop()
		for {
			select {
			case <-c.stopChan:
				return
			case <-ticker.C:
				c.cleanExpired()
			}
		}
	}()
}

func (c *Cleaner) cleanExpired() {
}

func (c *Cleaner) Stop() {
	close(c.stopChan)
	c.wg.Wait()
}
