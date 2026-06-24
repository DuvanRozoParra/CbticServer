package colors

import (
	"sync"
	"testing"
	"time"
)

func TestPool_Stress50Goroutines(t *testing.T) {
	if testing.Short() {
		t.Skip("skip stress test en -short")
	}
	p := NewPool()

	var wg sync.WaitGroup
	const goroutines = 50
	const opsPerG = 5

	start := time.Now()
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			local := make([]string, 0, opsPerG)
			for i := 0; i < opsPerG; i++ {
				c, err := p.Acquire()
				if err != nil {
					t.Errorf("g=%d op=%d: %v", id, i, err)
					return
				}
				local = append(local, c)
			}
			for _, c := range local {
				p.Release(c)
			}
		}(g)
	}
	wg.Wait()
	elapsed := time.Since(start)

	total := goroutines * opsPerG
	t.Logf("OK %d acquires+releases en %v (%.0f ops/s)",
		total*2, elapsed, float64(total*2)/elapsed.Seconds())
}
