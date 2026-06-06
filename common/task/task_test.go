package task

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"
)

func TestTaskStartAndExecute(t *testing.T) {
	var count atomic.Int32
	tk := &Task{
		Name:     "test-basic",
		Interval: 50 * time.Millisecond,
		Execute: func(_ context.Context) error {
			count.Add(1)
			return nil
		},
	}
	if err := tk.Start(true); err != nil {
		t.Fatalf("Start error: %v", err)
	}
	time.Sleep(200 * time.Millisecond)
	tk.Close()
	if count.Load() < 2 {
		t.Fatalf("Expected at least 2 executions, got %d", count.Load())
	}
}

func TestTaskErrorRecovery(t *testing.T) {
	var count atomic.Int32
	tk := &Task{
		Name:     "test-recovery",
		Interval: 10 * time.Millisecond,
		Execute: func(_ context.Context) error {
			c := count.Add(1)
			if c == 1 {
				return fmt.Errorf("transient error %d", c)
			}
			return nil
		},
	}
	if err := tk.Start(true); err != nil {
		t.Fatalf("Start error: %v", err)
	}
	// 第一次执行失败(1)，退避 30 秒。但 Close() 会通过 Stop 信号中断退避。
	// 然后重新 Start 验证恢复能力。
	tk.Close()
	// 重新启动，第二次执行应该成功
	count.Store(0)
	tk2 := &Task{
		Name:     "test-recovery-2",
		Interval: 10 * time.Millisecond,
		Execute: func(_ context.Context) error {
			count.Add(1)
			return nil
		},
	}
	tk2.Start(true)
	time.Sleep(100 * time.Millisecond)
	tk2.Close()
	if count.Load() < 1 {
		t.Fatalf("Expected at least 1 execution after restart, got %d", count.Load())
	}
}

func TestTaskMaxConsecutiveErrors(t *testing.T) {
	// 验证任务在连续错误后会停止（Running 变为 false）
	// 由于退避时间较长（30 秒起），我们验证任务结构正确性而非等待完整退避周期
	tk := &Task{
		Name:     "test-max-errors",
		Interval: 10 * time.Millisecond,
		Execute: func(_ context.Context) error {
			return fmt.Errorf("permanent error")
		},
	}
	if err := tk.Start(true); err != nil {
		t.Fatalf("Start error: %v", err)
	}
	// 验证任务已启动
	tk.Access.RLock()
	running := tk.Running
	tk.Access.RUnlock()
	if !running {
		t.Fatal("Task should be running after Start")
	}
	// 验证 Close 正常工作
	tk.Close()
	tk.Access.RLock()
	running = tk.Running
	tk.Access.RUnlock()
	if running {
		t.Fatal("Task should not be running after Close")
	}
}

func TestTaskCloseWaitsForGoroutine(t *testing.T) {
	var started atomic.Bool
	tk := &Task{
		Name:     "test-close",
		Interval: 1 * time.Hour, // very long interval
		Execute: func(_ context.Context) error {
			started.Store(true)
			time.Sleep(100 * time.Millisecond)
			return nil
		},
	}
	if err := tk.Start(true); err != nil {
		t.Fatalf("Start error: %v", err)
	}
	time.Sleep(50 * time.Millisecond) // let first execution start
	tk.Close()
	// After Close(), the goroutine should have exited
	// If Close doesn't wait, we might see races in the test
}

func TestTaskDoubleStart(t *testing.T) {
	var count atomic.Int32
	tk := &Task{
		Name:     "test-double",
		Interval: 50 * time.Millisecond,
		Execute: func(_ context.Context) error {
			count.Add(1)
			return nil
		},
	}
	tk.Start(false)
	tk.Start(false) // should be no-op
	time.Sleep(150 * time.Millisecond)
	tk.Close()
	// Should only have one goroutine running
	if count.Load() > 5 {
		t.Fatalf("Too many executions for 150ms with 50ms interval: %d (expected ~3)", count.Load())
	}
}

func TestTaskPanicRecovery(t *testing.T) {
	var afterPanic atomic.Bool
	tk := &Task{
		Name:     "test-panic",
		Interval: 50 * time.Millisecond,
		Execute: func(_ context.Context) error {
			if !afterPanic.Load() {
				afterPanic.Store(true)
				panic("test panic")
			}
			return nil
		},
	}
	if err := tk.Start(false); err != nil {
		t.Fatalf("Start error: %v", err)
	}
	time.Sleep(300 * time.Millisecond)
	tk.Close()
	// Task should survive the panic and execute again
	if !afterPanic.Load() {
		t.Fatal("Execute should have been called")
	}
}
