package typegame

import "sync"

type PlayersArray struct {
	Players []Player `json:"players"`
}

type Player struct {
	Id        string   `json:"id"`
	Head      BodyPart `json:"head"`
	Body      BodyPart `json:"body"`
	HandLeft  BodyPart `json:"handLeft"`
	HandRight BodyPart `json:"handRight"`
}

type BodyPart struct {
	Position Vector3    `json:"position"`
	Rotation Quaternion `json:"rotation"`
}

type Vector3 struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	Z float64 `json:"z"`
}

type Quaternion struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	Z float64 `json:"z"`
	W float64 `json:"w"`
}

type IdentifyManager struct {
	Colors    []string
	Remaining []string
	Mu        sync.RWMutex
}

type AddPlayerMsg struct {
	Id    string `json:"id"`
	Color string `json:"color"`
}
