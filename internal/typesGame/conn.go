package typegame

import (
	"github.com/DuvanRozoParra/servercbtic/internal/config"
)

type MessageObject struct {
	Data  string             `json:"data"`
	From  string             `json:"from"`
	Event config.EventServer `json:"events"`
}
