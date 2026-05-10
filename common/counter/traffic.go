package counter

import (
	"sync"
	"sync/atomic"
)

type TrafficCounter struct {
	Counters sync.Map
	// 最近使用的计数器，减少 sync.Map 的查找
	cache sync.Map // string -> *TrafficStorage
}

type TrafficStorage struct {
	UpCounter   atomic.Int64
	DownCounter atomic.Int64
}

func NewTrafficCounter() *TrafficCounter {
	return &TrafficCounter{}
}

func (c *TrafficCounter) GetCounter(uuid string) *TrafficStorage {
	// 先检查缓存
	if cts, ok := c.cache.Load(uuid); ok {
		return cts.(*TrafficStorage)
	}
	// 缓存未命中，从主 map 中获取
	if cts, ok := c.Counters.Load(uuid); ok {
		c.cache.Store(uuid, cts)
		return cts.(*TrafficStorage)
	}
	newStorage := &TrafficStorage{}
	if cts, loaded := c.Counters.LoadOrStore(uuid, newStorage); loaded {
		c.cache.Store(uuid, cts)
		return cts.(*TrafficStorage)
	}
	c.cache.Store(uuid, newStorage)
	return newStorage
}

func (c *TrafficCounter) GetUpCount(uuid string) int64 {
	if cts, ok := c.cache.Load(uuid); ok {
		return cts.(*TrafficStorage).UpCounter.Load()
	}
	if cts, ok := c.Counters.Load(uuid); ok {
		c.cache.Store(uuid, cts)
		return cts.(*TrafficStorage).UpCounter.Load()
	}
	return 0
}

func (c *TrafficCounter) GetDownCount(uuid string) int64 {
	if cts, ok := c.cache.Load(uuid); ok {
		return cts.(*TrafficStorage).DownCounter.Load()
	}
	if cts, ok := c.Counters.Load(uuid); ok {
		c.cache.Store(uuid, cts)
		return cts.(*TrafficStorage).DownCounter.Load()
	}
	return 0
}

func (c *TrafficCounter) Len() int {
	length := 0
	c.Counters.Range(func(_, _ interface{}) bool {
		length++
		return true
	})
	return length
}

func (c *TrafficCounter) Reset(uuid string) {
	if cts, ok := c.cache.Load(uuid); ok {
		cts.(*TrafficStorage).UpCounter.Store(0)
		cts.(*TrafficStorage).DownCounter.Store(0)
		return
	}
	if cts, ok := c.Counters.Load(uuid); ok {
		cts.(*TrafficStorage).UpCounter.Store(0)
		cts.(*TrafficStorage).DownCounter.Store(0)
	}
}

func (c *TrafficCounter) Delete(uuid string) {
	c.cache.Delete(uuid)
	c.Counters.Delete(uuid)
}

func (c *TrafficCounter) Rx(uuid string, n int) {
	cts := c.GetCounter(uuid)
	cts.DownCounter.Add(int64(n))
}

func (c *TrafficCounter) Tx(uuid string, n int) {
	cts := c.GetCounter(uuid)
	cts.UpCounter.Add(int64(n))
}
