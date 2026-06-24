package config

import (
	"os"
	"strconv"
	"time"
)

type EventServer int

const (
	RayInteraction EventServer = iota
	MovePlayer
	ActionHandsPlayer

	AddPlayer
	UpdatePlayer
	RemovePlayer
	IdentifyPlayer
	ServerShutdown
)

var (
	Address           = envStr("CBTIC_ADDRESS", ":8080")
	QueueBufferSize   = envInt("CBTIC_QUEUE_BUFFER", 5000)
	QueueWorkerSize   = envInt("CBTIC_QUEUE_WORKERS", 12)
	OutboxSize        = envInt("CBTIC_OUTBOX_SIZE", 256)
	WriteTimeout      = envDur("CBTIC_WRITE_TIMEOUT", 2*time.Second)
	ReadTimeout       = envDur("CBTIC_READ_TIMEOUT", 60*time.Second)
	ShutdownTimeout   = envDur("CBTIC_SHUTDOWN_TIMEOUT", 10*time.Second)
	IngressRatePerSec = envInt("CBTIC_INGRESS_RATE", 200)
	LogLevel          = envStr("CBTIC_LOG_LEVEL", "info")
)

func envStr(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func envDur(key string, def time.Duration) time.Duration {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}
