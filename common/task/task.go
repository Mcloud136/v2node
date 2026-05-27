package task

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
)

type Task struct {
	Name     string
	Interval time.Duration
	Execute  func(context.Context) error
	Access   sync.RWMutex
	Running  bool
	ReloadCh chan struct{}
	Stop     chan struct{}
	done     chan struct{}
}

func (t *Task) Start(first bool) error {
	t.Access.Lock()
	if t.Running {
		t.Access.Unlock()
		return nil
	}
	t.Running = true
	t.Stop = make(chan struct{})
	t.done = make(chan struct{})
	t.Access.Unlock()
	go func() {
		defer func() {
			t.Access.Lock()
			t.Running = false
			t.Access.Unlock()
			close(t.done)
		}()
		timer := time.NewTimer(t.Interval)
		defer timer.Stop()
		if first {
			if err := t.ExecuteWithTimeout(); err != nil {
				log.Errorf("Task %s first execution error: %v", t.Name, err)
			}
		}

		consecutiveErrors := 0
		for {
			timer.Reset(t.Interval)
			select {
			case <-timer.C:
				// continue
			case <-t.Stop:
				return
			}

			if err := t.ExecuteWithTimeout(); err != nil {
				consecutiveErrors++
				log.Errorf("Task %s execution error (consecutive: %d): %v", t.Name, consecutiveErrors, err)
				// 指数退避：错误越多等待越久，但不超过 5 分钟
				backoff := time.Duration(consecutiveErrors) * 30 * time.Second
				if backoff > 5*time.Minute {
					backoff = 5 * time.Minute
				}
				select {
				case <-time.After(backoff):
				case <-t.Stop:
					return
				}
			} else {
				consecutiveErrors = 0
			}
		}
	}()

	return nil
}

func (t *Task) ExecuteWithTimeout() (retErr error) {
	ctx, cancel := context.WithTimeout(context.Background(), min(5*t.Interval, 5*time.Minute))
	defer cancel()
	done := make(chan error, 1)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				done <- fmt.Errorf("panic in task %s: %v", t.Name, r)
			}
		}()
		done <- t.Execute(ctx)
	}()

	select {
	case <-ctx.Done():
		log.Errorf("Task %s execution timed out, reloading", t.Name)
		if t.ReloadCh != nil {
			select {
			case t.ReloadCh <- struct{}{}:
			default:
			}
		} else {
			log.Error("Reload channel is nil, cannot trigger reload")
		}
		return nil
	case err := <-done:
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil
		}
		return err
	}
}

func (t *Task) safeStop() {
	t.Access.Lock()
	if t.Running {
		t.Running = false
		close(t.Stop)
	}
	t.Access.Unlock()
}

func (t *Task) Close() {
	t.safeStop()
	if t.done != nil {
		<-t.done
	}
	log.Warningf("Task %s stopped", t.Name)
}
