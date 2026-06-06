package counter

import (
	"sync"
	"testing"
)

func TestTrafficCounterBasic(t *testing.T) {
	tc := NewTrafficCounter()
	ts := tc.GetCounter("user1")
	ts.UpCounter.Add(100)
	ts.DownCounter.Add(200)

	if ts.UpCounter.Load() != 100 {
		t.Fatalf("Expected UpCounter=100, got %d", ts.UpCounter.Load())
	}
	if ts.DownCounter.Load() != 200 {
		t.Fatalf("Expected DownCounter=200, got %d", ts.DownCounter.Load())
	}
}

func TestTrafficCounterSwapZero(t *testing.T) {
	tc := NewTrafficCounter()
	ts := tc.GetCounter("user1")
	ts.UpCounter.Add(500)
	ts.DownCounter.Add(300)

	up := ts.UpCounter.Swap(0)
	down := ts.DownCounter.Swap(0)

	if up != 500 {
		t.Fatalf("Expected swap up=500, got %d", up)
	}
	if down != 300 {
		t.Fatalf("Expected swap down=300, got %d", down)
	}
	if ts.UpCounter.Load() != 0 {
		t.Fatalf("Expected UpCounter=0 after swap, got %d", ts.UpCounter.Load())
	}
}

func TestTrafficCounterAddBack(t *testing.T) {
	tc := NewTrafficCounter()
	ts := tc.GetCounter("user1")
	ts.UpCounter.Add(500)

	// Simulate: Swap, check threshold, add back if below
	up := ts.UpCounter.Swap(0)
	if up < 1000 {
		ts.UpCounter.Add(up) // add back
	}

	if ts.UpCounter.Load() != 500 {
		t.Fatalf("Expected UpCounter=500 after add-back, got %d", ts.UpCounter.Load())
	}
}

func TestTrafficCounterDelete(t *testing.T) {
	tc := NewTrafficCounter()
	tc.GetCounter("user1")
	tc.Delete("user1")
	tc.GetCounter("user1") // should create new

	ts := tc.GetCounter("user1")
	if ts.UpCounter.Load() != 0 {
		t.Fatal("New counter should be zero")
	}
}

func TestTrafficCounterConcurrent(t *testing.T) {
	tc := NewTrafficCounter()
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ts := tc.GetCounter("user1")
			ts.UpCounter.Add(1)
			ts.DownCounter.Add(1)
		}()
	}
	wg.Wait()
	ts := tc.GetCounter("user1")
	if ts.UpCounter.Load() != 100 {
		t.Fatalf("Expected 100, got %d", ts.UpCounter.Load())
	}
}
