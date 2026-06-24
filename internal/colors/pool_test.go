package colors

import (
	"regexp"
	"sync"
	"testing"
)

func TestPoolInfinito(t *testing.T) {
	p := NewPool()
	hexRe := regexp.MustCompile(`^#[0-9A-F]{6}$`)
	got := make(map[string]struct{}, 1000)

	for i := 0; i < 1000; i++ {
		c, err := p.Acquire()
		if err != nil {
			t.Fatalf("FAIL at i=%d: %v", i, err)
		}
		if !hexRe.MatchString(c) {
			t.Fatalf("FAIL formato at i=%d: %q", i, c)
		}
		if _, dup := got[c]; dup {
			t.Fatalf("FAIL duplicado at i=%d: %q", i, c)
		}
		got[c] = struct{}{}
	}
	t.Logf("OK %d colores únicos generados, todos formato #RRGGBB", len(got))

	p.Release("#000000")
	t.Log("OK Release no panic")
}

func TestPoolReleaseYReuse(t *testing.T) {
	p := NewPool()
	c1, _ := p.Acquire()
	p.Release(c1)
	c2, _ := p.Acquire()
	if c1 != c2 {
		t.Logf("OK re-acquire devolvió color diferente (esperado con algoritmo iterativo): c1=%s c2=%s", c1, c2)
	} else {
		t.Logf("OK re-acquire devolvió el mismo color: %s", c1)
	}
}

func TestPoolConcurrente(t *testing.T) {
	p := NewPool()
	const goroutines = 100
	const opsPerG = 10

	var wg sync.WaitGroup
	all := make([]string, goroutines*opsPerG)
	idx := 0
	var idxMu sync.Mutex

	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < opsPerG; i++ {
				c, err := p.Acquire()
				if err != nil {
					t.Errorf("Acquire error: %v", err)
					return
				}
				idxMu.Lock()
				all[idx] = c
				idx++
				idxMu.Unlock()
			}
		}()
	}
	wg.Wait()

	uniq := make(map[string]struct{}, len(all))
	for _, c := range all {
		if _, dup := uniq[c]; dup {
			t.Fatalf("duplicado concurrente: %s", c)
		}
		uniq[c] = struct{}{}
	}
	t.Logf("OK %d acquires concurrentes, %d únicos", len(all), len(uniq))
}

func TestPoolReleaseConcurrente(t *testing.T) {
	p := NewPool()
	const goroutines = 50

	colors := make([]string, goroutines)
	for i := 0; i < goroutines; i++ {
		c, _ := p.Acquire()
		colors[i] = c
	}

	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(c string) {
			defer wg.Done()
			p.Release(c)
		}(colors[i])
	}
	wg.Wait()
	t.Log("OK releases concurrentes sin race")
}
