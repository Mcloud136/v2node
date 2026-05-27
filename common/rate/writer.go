package rate

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/juju/ratelimit"
	"github.com/xtls/xray-core/common"
	"github.com/xtls/xray-core/common/buf"
)

// 为 Writer 和 DynamicBucket 添加对象池以减少分配
var writerPool = sync.Pool{
	New: func() interface{} {
		return &Writer{}
	},
}

type Writer struct {
	writer  buf.Writer
	limiter *DynamicBucket
}

type DynamicBucket struct {
	v atomic.Value // *ratelimit.Bucket
}

func NewDynamicBucket(rate int64) *DynamicBucket {
	b := ratelimit.NewBucketWithQuantum(time.Second, rate, rate)
	d := &DynamicBucket{}
	d.v.Store(b)
	return d
}

func (d *DynamicBucket) Get() *ratelimit.Bucket {
	return d.v.Load().(*ratelimit.Bucket)
}

func (d *DynamicBucket) Update(rate int64) {
	newB := ratelimit.NewBucketWithQuantum(time.Second, rate, rate)
	d.v.Store(newB)
}

func NewRateLimitWriter(writer buf.Writer, limiter *DynamicBucket) buf.Writer {
	w := writerPool.Get().(*Writer)
	w.writer = writer
	w.limiter = limiter
	return w
}

func (w *Writer) Close() error {
	err := common.Close(w.writer)
	// 清理字段并放回池中
	w.writer = nil
	w.limiter = nil
	writerPool.Put(w)
	return err
}

func (w *Writer) WriteMultiBuffer(mb buf.MultiBuffer) error {
	limiter := w.limiter.Get()
	if limiter != nil {
		remaining := int64(mb.Len())
		for remaining > 0 {
			taken := limiter.TakeAvailable(remaining)
			if taken > 0 {
				remaining -= taken
				continue
			}
			// 批量等待：每次最多等待 4096 bytes 的 token，减少循环次数
			batchSize := remaining
			if batchSize > 4096 {
				batchSize = 4096
			}
			limiter.Wait(batchSize)
			remaining -= batchSize
		}
	}
	return w.writer.WriteMultiBuffer(mb)
}
