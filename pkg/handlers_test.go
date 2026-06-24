package pkg

import (
	"testing"

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

func TestSendMsg_NilPayloadNoBloquea(t *testing.T) {
	jg := newTestJobGame()

	py := &typegame.Players{
		Id:     "x",
		Outbox: make(chan typegame.OutboundFrame, 10),
		Stop:   make(chan struct{}),
	}
	jg.Players["x"] = py

	SendMsg(jg, "x", 1, nil)
	select {
	case <-py.Outbox:
		t.Fatal("no debería haber encolado nada con payload nil")
	default:
	}
}

func TestSendAllMsg_NilPayloadNoBloquea(t *testing.T) {
	jg := newTestJobGame()

	py := &typegame.Players{
		Id:     "x",
		Outbox: make(chan typegame.OutboundFrame, 10),
		Stop:   make(chan struct{}),
	}
	jg.Players["x"] = py

	SendAllMsg(jg, 1, nil)
	select {
	case <-py.Outbox:
		t.Fatal("no debería haber encolado nada con payload nil")
	default:
	}
}

func TestSendAllExcept_NoEnviaAlExcluido(t *testing.T) {
	jg := newTestJobGame()

	pyA := &typegame.Players{
		Id:     "a",
		Outbox: make(chan typegame.OutboundFrame, 10),
		Stop:   make(chan struct{}),
	}
	pyB := &typegame.Players{
		Id:     "b",
		Outbox: make(chan typegame.OutboundFrame, 10),
		Stop:   make(chan struct{}),
	}
	pyC := &typegame.Players{
		Id:     "c",
		Outbox: make(chan typegame.OutboundFrame, 10),
		Stop:   make(chan struct{}),
	}
	jg.Players["a"] = pyA
	jg.Players["b"] = pyB
	jg.Players["c"] = pyC

	SendAllExcept(jg, "b", 1, []byte("hello"))

	select {
	case <-pyA.Outbox:
	default:
		t.Fatal("a debería haber recibido mensaje")
	}
	select {
	case <-pyB.Outbox:
		t.Fatal("b NO debería haber recibido (está excluido)")
	default:
	}
	select {
	case <-pyC.Outbox:
	default:
		t.Fatal("c debería haber recibido mensaje")
	}
}

func TestSendAllExcept_NilPayloadNoBloquea(t *testing.T) {
	jg := newTestJobGame()

	py := &typegame.Players{
		Id:     "x",
		Outbox: make(chan typegame.OutboundFrame, 10),
		Stop:   make(chan struct{}),
	}
	jg.Players["x"] = py

	SendAllExcept(jg, "y", 1, nil)
	select {
	case <-py.Outbox:
		t.Fatal("no debería haber encolado nada con payload nil")
	default:
	}
}

func TestSendAllMsg_OutboxFullDropLog(t *testing.T) {
	jg := newTestJobGame()

	py := &typegame.Players{
		Id:     "x",
		Outbox: make(chan typegame.OutboundFrame, 1),
		Stop:   make(chan struct{}),
	}
	jg.Players["x"] = py

	SendAllMsg(jg, 1, []byte("primero"))
	SendAllMsg(jg, 1, []byte("segundo"))

	select {
	case <-py.Outbox:
	default:
		t.Fatal("esperaba primer mensaje en outbox")
	}
}

func TestSendMsg_JugadorNoExisteSilencioso(t *testing.T) {
	jg := newTestJobGame()
	SendMsg(jg, "fantasma", 1, []byte("hola"))
}

func TestAddUser_IdDuplicadoRechaza(t *testing.T) {
	jg := newTestJobGame()

	py := &typegame.Players{Id: "dup"}
	jg.Players["dup"] = py

	_, _, err := AddUser(jg, nil, "dup")
	if err == nil {
		t.Fatal("esperaba error por id duplicado")
	}
}

func TestAddUser_ColorsNilRechaza(t *testing.T) {
	jg := &typegame.JobGame{
		Players: make(map[string]*typegame.Players),
		Queue:   make(chan typegame.MessageObject, 10),
		Colors:  nil,
	}

	_, _, err := AddUser(jg, nil, "nuevo")
	if err == nil {
		t.Fatal("esperaba error por Colors nil")
	}
}

func TestDesconnectUser_ReleaseColorYNilGuard(t *testing.T) {
	jg := newTestJobGame()
	color, _ := jg.Colors.Acquire()
	py := &typegame.Players{
		Id:     "x",
		Color:  color,
		Outbox: make(chan typegame.OutboundFrame, 1),
		Stop:   make(chan struct{}),
	}
	jg.Players["x"] = py

	DesconnectUser(jg, nil, "x")

	if _, exists := jg.Players["x"]; exists {
		t.Fatal("jugador debería estar eliminado del mapa")
	}
}

func TestKickPlayer_RemueveYLiberaColor(t *testing.T) {
	jg := newTestJobGame()
	color, _ := jg.Colors.Acquire()
	py := &typegame.Players{
		Id:     "old",
		Color:  color,
		Stop:   make(chan struct{}),
		Outbox: make(chan typegame.OutboundFrame, 1),
	}
	jg.Players["old"] = py

	if !KickPlayer(jg, "old") {
		t.Fatal("KickPlayer debería retornar true para jugador existente")
	}
	if _, exists := jg.Players["old"]; exists {
		t.Fatal("jugador debería estar eliminado del mapa tras kick")
	}
	select {
	case <-py.Stop:
	default:
		t.Fatal("Stop debería estar cerrado tras kick")
	}
}

func TestKickPlayer_NoExisteRetornaFalse(t *testing.T) {
	jg := newTestJobGame()
	if KickPlayer(jg, "fantasma") {
		t.Fatal("KickPlayer debería retornar false para jugador inexistente")
	}
}

func TestAddUser_TrasKickEsExitoso(t *testing.T) {
	jg := newTestJobGame()

	_, _, err := AddUser(jg, nil, "dup")
	if err != nil {
		t.Fatalf("primer AddUser debería exitar: %v", err)
	}

	_, _, err = AddUser(jg, nil, "dup")
	if err == nil {
		t.Fatal("segundo AddUser debería fallar por duplicado")
	}

	if !KickPlayer(jg, "dup") {
		t.Fatal("KickPlayer debería tener éxito")
	}

	_, _, err = AddUser(jg, nil, "dup")
	if err != nil {
		t.Fatalf("AddUser tras kick debería exitar: %v", err)
	}
}

func TestSafeSendOutbox_ClosedChannelNoPanic(t *testing.T) {
	py := &typegame.Players{
		Id:     "closed",
		Outbox: make(chan typegame.OutboundFrame, 1),
		Stop:   make(chan struct{}),
	}
	close(py.Outbox)

	safeSendOutbox(py, typegame.OutboundFrame{Opcode: 1, Data: []byte("test")})
}

func TestSafeSendOutbox_FullChannelNoBlock(t *testing.T) {
	py := &typegame.Players{
		Id:     "full",
		Outbox: make(chan typegame.OutboundFrame, 1),
		Stop:   make(chan struct{}),
	}

	safeSendOutbox(py, typegame.OutboundFrame{Opcode: 1, Data: []byte("1")})
	safeSendOutbox(py, typegame.OutboundFrame{Opcode: 1, Data: []byte("2")})
}

func TestConfigEventConstantesIntactos(t *testing.T) {
	if int(config.RayInteraction) != 0 {
		t.Fatalf("RayInteraction debe ser 0, got %d", int(config.RayInteraction))
	}
	if int(config.MovePlayer) != 1 {
		t.Fatalf("MovePlayer debe ser 1, got %d", int(config.MovePlayer))
	}
	if int(config.ActionHandsPlayer) != 2 {
		t.Fatalf("ActionHandsPlayer debe ser 2, got %d", int(config.ActionHandsPlayer))
	}
	if int(config.AddPlayer) != 3 {
		t.Fatalf("AddPlayer debe ser 3, got %d", int(config.AddPlayer))
	}
	if int(config.RemovePlayer) != 5 {
		t.Fatalf("RemovePlayer debe ser 5, got %d", int(config.RemovePlayer))
	}
	if int(config.ServerShutdown) != 7 {
		t.Fatalf("ServerShutdown debe ser 7, got %d", int(config.ServerShutdown))
	}
}
