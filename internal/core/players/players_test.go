package players

import (
	"testing"

	typegame "github.com/DuvanRozoParra/servercbtic/internal/typesGame"
)

func TestConvertToPlayer_Valido(t *testing.T) {
	data := `{"id":"","head":{"position":{"x":1,"y":2,"z":3},"rotation":{"x":0,"y":0,"z":0,"w":1}},"body":{"position":{"x":0,"y":0,"z":0},"rotation":{"x":0,"y":0,"z":0,"w":1}},"handLeft":{"position":{"x":0,"y":0,"z":0},"rotation":{"x":0,"y":0,"z":0,"w":1}},"handRight":{"position":{"x":0,"y":0,"z":0},"rotation":{"x":0,"y":0,"z":0,"w":1}}}`

	p := ConvertToPlayer(data)
	if p == nil {
		t.Fatal("expected non-nil player")
	}
	if p.Head.Position.X != 1 || p.Head.Position.Y != 2 || p.Head.Position.Z != 3 {
		t.Fatalf("head position mal: %+v", p.Head.Position)
	}
}

func TestConvertToPlayer_Invalido(t *testing.T) {
	p := ConvertToPlayer("not json")
	if p != nil {
		t.Fatalf("expected nil, got %+v", p)
	}
}

func TestConvertToPlayer_Vacio(t *testing.T) {
	p := ConvertToPlayer("")
	if p != nil {
		t.Fatalf("expected nil para string vacío")
	}
}

func TestGetAllPlayerId_TodosIncluidos(t *testing.T) {
	jg := &typegame.JobGame{
		Players: make(map[string]*typegame.Players),
	}
	jg.Players["a"] = &typegame.Players{Id: "a", Color: "#FF0000"}
	jg.Players["b"] = &typegame.Players{Id: "b", Color: "#00FF00"}

	result := GetAllPlayerId(jg, "a")

	if len(result) != 2 {
		t.Fatalf("esperaba 2 jugadores (incluye emisor), got %d", len(result))
	}
}

func TestEventMovement_NilSiPayloadInvalido(t *testing.T) {
	jg := &typegame.JobGame{
		Players: make(map[string]*typegame.Players),
	}
	jg.Players["a"] = &typegame.Players{Id: "a", Player: &typegame.Player{}}

	result := EventMovement(jg, "a", "invalid json")
	if result != nil {
		t.Fatalf("esperaba nil con payload inválido, got %+v", result)
	}
}

func TestEventMovement_NilSiJugadorNoExiste(t *testing.T) {
	jg := &typegame.JobGame{
		Players: make(map[string]*typegame.Players),
	}
	data := `{"id":"","head":{"position":{"x":0,"y":0,"z":0},"rotation":{"x":0,"y":0,"z":0,"w":1}},"body":{"position":{"x":0,"y":0,"z":0},"rotation":{"x":0,"y":0,"z":0,"w":1}},"handLeft":{"position":{"x":0,"y":0,"z":0},"rotation":{"x":0,"y":0,"z":0,"w":1}},"handRight":{"position":{"x":0,"y":0,"z":0},"rotation":{"x":0,"y":0,"z":0,"w":1}}}`

	result := EventMovement(jg, "nonexistent", data)
	if result != nil {
		t.Fatalf("esperaba nil con jugador inexistente")
	}
}

func TestEventMovement_DevuelveUnSoloPlayerConIdYOffset(t *testing.T) {
	jg := &typegame.JobGame{
		Players: make(map[string]*typegame.Players),
	}
	data := `{"id":"","head":{"position":{"x":0,"y":5,"z":0},"rotation":{"x":0,"y":0,"z":0,"w":1}},"body":{"position":{"x":0,"y":3,"z":0},"rotation":{"x":0,"y":0,"z":0,"w":1}},"handLeft":{"position":{"x":0,"y":0,"z":0},"rotation":{"x":0,"y":0,"z":0,"w":1}},"handRight":{"position":{"x":0,"y":0,"z":0},"rotation":{"x":0,"y":0,"z":0,"w":1}}}`
	jg.Players["alice"] = &typegame.Players{Id: "alice", Player: &typegame.Player{}}

	result := EventMovement(jg, "alice", data)
	if result == nil {
		t.Fatal("esperaba resultado no-nil")
	}
	if result.Id != "alice" {
		t.Fatalf("esperaba Id='alice', got %q", result.Id)
	}
	expectedY := 3.0 - 0.3
	if result.Body.Position.Y != expectedY {
		t.Fatalf("esperaba Body.Y=%.1f (3-0.3), got %.1f", expectedY, result.Body.Position.Y)
	}
	if result.Head.Position.Y != 5 {
		t.Fatalf("Head.Y no debería tener offset, got %v", result.Head.Position.Y)
	}
}
