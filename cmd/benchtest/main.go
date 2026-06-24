package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"runtime"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/DuvanRozoParra/servercbtic/internal/config"
	"github.com/DuvanRozoParra/servercbtic/internal/network/queue"
	typegame "github.com/DuvanRozoParra/servercbtic/internal/typesGame"
)

var (
	clients    = flag.Int("clients", 50, "number of concurrent clients")
	duration   = flag.Duration("duration", 30*time.Second, "test duration")
	rate       = flag.Int("rate", 60, "messages per second per client")
	playerData = flag.String("player", `{"id":"","head":{"position":{"x":0,"y":1.6,"z":0},"rotation":{"x":0,"y":0,"z":0,"w":1}},"body":{"position":{"x":0,"y":1,"z":0},"rotation":{"x":0,"y":0,"z":0,"w":1}},"handLeft":{"position":{"x":-0.3,"y":1.2,"z":0.3},"rotation":{"x":0,"y":0,"z":0,"w":1}},"handRight":{"position":{"x":0.3,"y":1.2,"z":0.3},"rotation":{"x":0,"y":0,"z":0,"w":1}}}`, "player payload")
)

func main() {
	flag.Parse()

	go func() {
		log.Println("pprof en :6060")
		_ = http.ListenAndServe("localhost:6060", nil)
	}()

	fmt.Printf("=== Benchmark: %d clientes × %d msg/s × %v ===\n", *clients, *rate, *duration)

	jg := queue.New(config.QueueBufferSize)

	queue.StartWorkers(config.QueueWorkerSize, jg)

	var connected, sendCount, recvCount, errCount, dropCount int64
	var latencies []time.Duration
	var latMu sync.Mutex

	var wg sync.WaitGroup
	start := time.Now()
	stop := make(chan struct{})

	for i := 0; i < *clients; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			pid := fmt.Sprintf("bench-%d", id)
			p, err := addBenchUser(jg, pid)
			if err != nil {
				atomic.AddInt64(&errCount, 1)
				return
			}
			atomic.AddInt64(&connected, 1)
			defer desconnectBenchUser(jg, pid, p)

			startNoopWriter(p, &recvCount, &dropCount)

			interval := time.Second / time.Duration(*rate)
			ticker := time.NewTicker(interval)
			defer ticker.Stop()

			for {
				select {
				case <-stop:
					return
				case <-ticker.C:
					atomic.AddInt64(&sendCount, 1)
					data := []byte(fmt.Sprintf(`{"data":%q,"from":%q,"events":1}`, *playerData, pid))

					sendStart := time.Now()
					select {
					case jg.Queue <- typegame.MessageObject{
						Data:  string(data),
						From:  pid,
						Event: config.MovePlayer,
					}:
					default:
						atomic.AddInt64(&dropCount, 1)
					}
					latMu.Lock()
					latencies = append(latencies, time.Since(sendStart))
					latMu.Unlock()
				}
			}
		}(i)
	}

	time.Sleep(*duration)
	close(stop)
	wg.Wait()
	elapsed := time.Since(start)

	latMu.Lock()
	if len(latencies) > 0 {
		sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
		p50 := latencies[len(latencies)*50/100]
		p95 := latencies[len(latencies)*95/100]
		p99 := latencies[len(latencies)*99/100]
		fmt.Printf("Latencias: p50=%v p95=%v p99=%v\n", p50, p95, p99)
	}
	latMu.Unlock()

	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	fmt.Printf("\n=== Resultados ===\n")
	fmt.Printf("Conectados:      %d/%d\n", connected, *clients)
	fmt.Printf("Errores:         %d\n", errCount)
	fmt.Printf("Enviados:        %d\n", sendCount)
	fmt.Printf("Recibidos:       %d\n", recvCount)
	fmt.Printf("Queue drops:     %d\n", dropCount)
	fmt.Printf("Duración:        %v\n", elapsed)
	fmt.Printf("Throughput:      %.0f msg/s (envío)\n", float64(sendCount)/elapsed.Seconds())
	fmt.Printf("Goroutines:      %d\n", runtime.NumGoroutine())
	fmt.Printf("HeapAlloc:       %d MB\n", m.HeapAlloc/1024/1024)
	fmt.Printf("HeapObjects:     %d\n", m.HeapObjects)
	fmt.Printf("TotalAlloc:      %d MB\n", m.TotalAlloc/1024/1024)
	fmt.Printf("NumGC:           %d\n", m.NumGC)

	fmt.Println()
	if connected == int64(*clients) {
		fmt.Println("✅ PASS: todos los clientes conectaron")
	} else {
		fmt.Printf("❌ FAIL: solo %d/%d conectaron\n", connected, *clients)
		os.Exit(1)
	}
}

func startNoopWriter(p *typegame.Players, recvCount, dropCount *int64) {
	go func() {
		for {
			select {
			case <-p.Stop:
				return
			case frame, ok := <-p.Outbox:
				if !ok {
					return
				}
				if frame.Data == nil {
					atomic.AddInt64(dropCount, 1)
					continue
				}
				atomic.AddInt64(recvCount, 1)
			}
		}
	}()
}

func addBenchUser(jg *typegame.JobGame, id string) (*typegame.Players, error) {
	jg.Mu.Lock()
	defer jg.Mu.Unlock()
	if _, ok := jg.Players[id]; ok {
		return nil, fmt.Errorf("dup")
	}
	color, _ := jg.Colors.Acquire()
	py := &typegame.Player{Id: id}
	player := &typegame.Players{
		Id:     id,
		Color:  color,
		Player: py,
		Outbox: make(chan typegame.OutboundFrame, config.OutboxSize),
		Stop:   make(chan struct{}),
	}
	data, _ := json.Marshal(typegame.AddPlayerMsg{Id: id, Color: color})
	jg.Players[id] = player
	jg.Queue <- typegame.MessageObject{Data: string(data), From: id, Event: config.AddPlayer}
	return player, nil
}

func desconnectBenchUser(jg *typegame.JobGame, id string, p *typegame.Players) {
	jg.Mu.Lock()
	if p.Color != "" && jg.Colors != nil {
		jg.Colors.Release(p.Color)
	}
	delete(jg.Players, id)
	if p.Stop != nil {
		close(p.Stop)
	}
	if p.Outbox != nil {
		close(p.Outbox)
	}
	jg.Mu.Unlock()
}
