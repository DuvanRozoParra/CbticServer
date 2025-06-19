package config

type EventServer int

const (
	Address         string = ":8080"
	QueueBufferSize int    = 5000
	QueueWorkerSize int    = 6
)

const (
	// client
	RayInteraction EventServer = iota
	MovePlayer
	ActionHandsPlayer

	// server
	AddPlayer
	UpdatePlayer
	RemovePlayer
	IdentifyPlayer
)
