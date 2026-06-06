package conf

import (
	"fmt"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	log "github.com/sirupsen/logrus"
)

func (p *Conf) Watch(filePath string, reload func()) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("new watcher error: %w", err)
	}

	var reloadMu sync.Mutex
	stopCh := make(chan struct{})

	go func() {
		var pre time.Time
		defer watcher.Close()
		for {
			select {
			case e := <-watcher.Events:
				if e.Has(fsnotify.Chmod) {
					continue
				}
				if pre.Add(10 * time.Second).After(time.Now()) {
					continue
				}
				pre = time.Now()
				go func() {
					debounceTimer := time.NewTimer(5 * time.Second)
					select {
					case <-debounceTimer.C:
					case <-stopCh:
						debounceTimer.Stop()
						return
					}
					reloadMu.Lock()
					defer reloadMu.Unlock()
					log.Info("config file changed, reloading...")
					newConf := New()
					err := newConf.LoadFromPath(filePath)
					if err != nil {
						log.WithField("err", err).Error("reload config error")
						return
					}
					// NOTE: *p = *newConf 不是原子操作。当前只有 watcher goroutine 写入 p，
					// 主循环在 reload() 中独立读取配置文件。如果未来添加并发读取 p 的路径，
					// 需要用 atomic.Pointer 或 mutex 保护。
					*p = *newConf
					reload()
					log.Info("reload config success")
				}()
			case err := <-watcher.Errors:
				if err != nil {
					log.WithField("err", err).Warn("file watcher error")
				}
			case <-stopCh:
				return
			}
		}
	}()

	err = watcher.Add(filePath)
	if err != nil {
		return fmt.Errorf("watch file error: %w", err)
	}
	return nil
}
