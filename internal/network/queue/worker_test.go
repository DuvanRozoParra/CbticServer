package queue

import (
	"sync"
	"testing"
	"time"

	"github.com/DuvanRozoParra/servercbtic/internal/colors"
	"github.com/DuvanRozoParra/servercbtic/internal/config"
	typegame "github.com/DuvanRozoParra/servercbtic/internal/typesGame"
)

func newTestJobGame() *typegame.JobGame {
	return &typegame.JobGame{
		Players: make(map[string]*typegame.Players),
		Queue:   make(chan typegame.MessageObject, 100),
		Colors:  colors.NewPool(),
	}
}

func TestWorker_LockLiberadoTrasError(t *testing.T) {
	jg := newTestJobGame()

	done := make(chan struct{})
	go func() {
		func() {
			defer func() {
				_ = recover()
			}()
			func() []byte {
				jg.Mu.Lock()
				defer jg.Mu.Unlock()

				time.Sleep(50 * time.Millisecond)
				return nil
			}()
		}()

		select {
		case <-time.After(100 * time.Millisecond):
			t.Error("Lock no se liberó tras defer Unlock")
		default:
		}
		close(done)
	}()

	_ = config.AddPlayer
	<-done
}

func TestWorker_MultiplesWorkersParalelos(t *testing.T) {
	jg := newTestJobGame()

	var wg sync.WaitGroup
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			func() []byte {
				jg.Mu.RLock()
				defer jg.Mu.RUnlock()
				time.Sleep(10 * time.Millisecond)
				return nil
			}()
		}()
	}
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Workers colgados (deadlock)")
	}
}

func TestWorker_LockDespuesDePanicSigueFuncional(t *testing.T) {
	jg := newTestJobGame()

	lockTaken := false
	lockReleased := false

	func() {
		defer func() {
			_ = recover()
		}()
		func() {
			jg.Mu.Lock()
			defer jg.Mu.Unlock()
			lockTaken = true
			panic("test panic mid-lock")
		}()
	}()

	if !lockTaken {
		t.Fatal("lock no se tomó")
	}

	done := make(chan struct{})
	go func() {
		jg.Mu.Lock()
		defer jg.Mu.Unlock()
		lockReleased = true
		close(done)
	}()

	select {
	case <-done:
		if !lockReleased {
			t.Fatal("lock no se liberó tras panic")
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("DEADLOCK: no se pudo tomar el lock tras panic (defer Unlock no se ejecutó)")
	}
}

func TestWorker_RecoverEnPanicNoCierraMutex(t *testing.T) {
	jg := newTestJobGame()

	func() {
		defer func() {
			_ = recover()
		}()
		func() {
			jg.Mu.Lock()
			defer jg.Mu.Unlock()
			panic("simulated")
		}()
	}()

	acquired := make(chan struct{})
	go func() {
		jg.Mu.RLock()
		defer jg.Mu.RUnlock()
		close(acquired)
	}()

	select {
	case <-acquired:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("mutex sigue tomado tras panic (defer Unlock NO se ejecutó)")
	}
}
