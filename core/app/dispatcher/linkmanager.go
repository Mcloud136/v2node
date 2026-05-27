package dispatcher

import (
	sync "sync"

	"github.com/xtls/xray-core/common"
	"github.com/xtls/xray-core/common/buf"
)

type ManagedWriter struct {
	writer  buf.Writer
	manager *LinkManager
}

func (w *ManagedWriter) WriteMultiBuffer(mb buf.MultiBuffer) error {
	return w.writer.WriteMultiBuffer(mb)
}

func (w *ManagedWriter) Close() error {
	w.manager.RemoveWriter(w)
	return common.Close(w.writer)
}

type LinkManager struct {
	links map[*ManagedWriter]buf.Reader
	mu    sync.RWMutex
}

func (m *LinkManager) AddLink(writer *ManagedWriter, reader buf.Reader) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.links[writer] = reader
}

func (m *LinkManager) RemoveWriter(writer *ManagedWriter) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.links, writer)
}

func (m *LinkManager) CloseAll() {
	m.mu.Lock()
	// 收集链接副本后释放锁，避免 ManagedWriter.Close() -> RemoveWriter() 再次加锁导致死锁
	links := make(map[*ManagedWriter]buf.Reader, len(m.links))
	for w, r := range m.links {
		links[w] = r
	}
	m.links = make(map[*ManagedWriter]buf.Reader)
	m.mu.Unlock()

	for w, r := range links {
		common.Close(w)
		common.Interrupt(r)
	}
}
