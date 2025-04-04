package server

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)


func TestConcurrencyLimiterFileOps(t *testing.T) {
	limiter := newConcurrencyLimiter(2, 100)

	var current int32
	var max int32
	var wg sync.WaitGroup

	simulate := func(method string) {
		defer wg.Done()
		release := limiter.acquire(method)
		atomic.AddInt32(&current, 1)
		c := atomic.LoadInt32(&current)
		if c > max {
			atomic.StoreInt32(&max, c)
		}

		// эмуляция работы (100 мс)
		time.Sleep(100 * time.Millisecond)
		atomic.AddInt32(&current, -1)
		release()
	}

	numCalls := 10
	wg.Add(numCalls)
	for range numCalls {
		go simulate("/Upload")
	}
	wg.Wait()

	if max > 2 {
		t.Errorf("Max concurrent fileOps = %d, expected <= 2", max)
	} else {
		t.Logf("Max concurrent fileOps = %d", max)
	}
}

func TestConcurrencyLimiterListOps(t *testing.T) {
	limiter := newConcurrencyLimiter(10, 3)

	var current int32
	var max int32
	var wg sync.WaitGroup

	simulate := func(method string) {
		defer wg.Done()
		release := limiter.acquire(method)
		atomic.AddInt32(&current, 1)
		c := atomic.LoadInt32(&current)
		if c > max {
			atomic.StoreInt32(&max, c)
		}
		// эмуляция работы (100 мс)
		time.Sleep(100 * time.Millisecond)
		atomic.AddInt32(&current, -1)
		release()
	}

	numCalls := 10
	wg.Add(numCalls)
	for range numCalls {
		go simulate("/ListFiles")
	}
	wg.Wait()

	if max > 3 {
		t.Errorf("Max concurrent listOps = %d, expected <= 3", max)
	} else {
		t.Logf("Max concurrent listOps = %d", max)
	}
}
