package pkg

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/DuvanRozoParra/servercbtic/internal/config"
	typegame "github.com/DuvanRozoParra/servercbtic/internal/typesGame"
	"github.com/gofiber/contrib/websocket"
	"github.com/rs/zerolog/log"
)

func DesconnectUser(jg *typegame.JobGame, c *websocket.Conn, id string) typegame.MessageObject {
	jg.Mu.Lock()
	if p, ok := jg.Players[id]; ok {
		if p.Color != "" && jg.Colors != nil {
			jg.Colors.Release(p.Color)
		}
		if p.Stop != nil {
			close(p.Stop)
		}
		if p.Outbox != nil {
			close(p.Outbox)
		}
	}
	delete(jg.Players, id)
	jg.Mu.Unlock()

	return typegame.MessageObject{
		Data:  "",
		From:  id,
		Event: config.RemovePlayer,
	}
}

func KickPlayer(jg *typegame.JobGame, id string) bool {
	jg.Mu.Lock()
	defer jg.Mu.Unlock()

	old, ok := jg.Players[id]
	if !ok || old == nil {
		return false
	}
	if old.Color != "" && jg.Colors != nil {
		jg.Colors.Release(old.Color)
	}
	if old.Stop != nil {
		close(old.Stop)
	}
	if old.Conn != nil {
		_ = old.Conn.SetReadDeadline(time.Now())
	}
	delete(jg.Players, id)
	return true
}

func AddUser(jg *typegame.JobGame, c *websocket.Conn, idPlayer string) (*typegame.MessageObject, *typegame.Players, error) {
	jg.Mu.Lock()
	defer jg.Mu.Unlock()

	if _, exists := jg.Players[idPlayer]; exists {
		return nil, nil, fmt.Errorf("id duplicado: %s", idPlayer)
	}

	if jg.Colors == nil {
		return nil, nil, fmt.Errorf("pool de colores no inicializado")
	}

	color, err := jg.Colors.Acquire()
	if err != nil {
		return nil, nil, fmt.Errorf("sin colores disponibles: %w", err)
	}
	py := &typegame.Player{}
	player := &typegame.Players{
		Id:     idPlayer,
		Color:  color,
		Conn:   c,
		Player: py,
		Outbox: make(chan typegame.OutboundFrame, config.OutboxSize),
		Stop:   make(chan struct{}),
	}

	jg.Players[idPlayer] = player

	return &typegame.MessageObject{From: idPlayer, Event: config.AddPlayer}, player, nil
}

func MarshalEventData(payload any) (string, error) {
	b, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func BuildWire(data, from string, event config.EventServer) ([]byte, error) {
	return json.Marshal(typegame.MessageObject{Data: data, From: from, Event: event})
}

func ByteToMessageObject(msg []byte) (typegame.MessageObject, error) {
	var message typegame.MessageObject
	err := json.Unmarshal(msg, &message)
	if err != nil {
		log.Warn().Err(err).Msg("Error al deserializar el mensaje")
		return typegame.MessageObject{}, err
	}
	return message, nil
}

func SendAllMsg(jg *typegame.JobGame, opcode int, msg []byte) {
	if msg == nil {
		log.Warn().Msg("SendAllMsg: payload nulo, broadcast descartado")
		return
	}

	jg.Mu.RLock()
	players := make([]*typegame.Players, 0, len(jg.Players))
	for _, p := range jg.Players {
		players = append(players, p)
	}
	jg.Mu.RUnlock()

	frame := typegame.OutboundFrame{Opcode: opcode, Data: msg}
	for _, p := range players {
		safeSendOutbox(p, frame)
	}
}

func SendAllExcept(jg *typegame.JobGame, exceptID string, opcode int, msg []byte) {
	if msg == nil {
		return
	}

	jg.Mu.RLock()
	players := make([]*typegame.Players, 0, len(jg.Players))
	for _, p := range jg.Players {
		if p.Id == exceptID {
			continue
		}
		players = append(players, p)
	}
	jg.Mu.RUnlock()

	frame := typegame.OutboundFrame{Opcode: opcode, Data: msg}
	for _, p := range players {
		safeSendOutbox(p, frame)
	}
}

func SendMsg(jg *typegame.JobGame, idPlayer string, opcode int, msg []byte) {
	if msg == nil {
		log.Warn().Str("player", idPlayer).Msg("SendMsg: payload nulo")
		return
	}

	jg.Mu.RLock()
	py, exists := jg.Players[idPlayer]
	jg.Mu.RUnlock()

	if !exists {
		return
	}

	safeSendOutbox(py, typegame.OutboundFrame{Opcode: opcode, Data: msg})
}

func safeSendOutbox(p *typegame.Players, frame typegame.OutboundFrame) {
	defer func() {
		if r := recover(); r != nil {
			log.Warn().Str("player", p.Id).Interface("panic", r).Msg("send on closed outbox")
		}
	}()
	select {
	case p.Outbox <- frame:
	default:
		log.Warn().Str("player", p.Id).Msg("outbox full, dropping msg")
	}
}

func StartWriter(p *typegame.Players) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Error().Str("player", p.Id).Interface("panic", r).Msg("writer panic, forzando cleanup")
				_ = p.Conn.SetReadDeadline(time.Now())
			}
		}()
		for {
			select {
			case frame, ok := <-p.Outbox:
				if !ok {
					return
				}
				if err := p.Conn.SetWriteDeadline(time.Now().Add(config.WriteTimeout)); err != nil {
					log.Warn().Err(err).Str("player", p.Id).Msg("writer: SetWriteDeadline")
					return
				}
				if err := p.Conn.WriteMessage(frame.Opcode, frame.Data); err != nil {
					log.Warn().Err(err).Str("player", p.Id).Msg("writer: WriteMessage")
					_ = p.Conn.SetReadDeadline(time.Now())
					return
				}
			case <-p.Stop:
				return
			}
		}
	}()
}
