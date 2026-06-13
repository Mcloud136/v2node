package rate

import (
	"sync/atomic"
	"time"

	"github.com/juju/ratelimit"
	"github.com/xtls/xray-core/common"
	"github.com/xtls/xray-core/common/buf"
)

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
	return &Writer{
		writer:  writer,
		limiter: limiter,
	}
}

func (w *Writer) Close() error {
	return common.Close(w.writer)
}

func (w *Writer) WriteMultiBuffer(mb buf.MultiBuffer) error {
	limiter := w.limiter.Get()
	if limiter != nil {
		limiter.Wait(int64(mb.Len()))
	}
	return w.writer.WriteMultiBuffer(mb)
}
