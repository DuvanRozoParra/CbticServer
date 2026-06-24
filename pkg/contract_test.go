package pkg

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/DuvanRozoParra/servercbtic/internal/config"
	typegame "github.com/DuvanRozoParra/servercbtic/internal/typesGame"
)

func TestBuildWire_EsJSONValido(t *testing.T) {
	cases := []struct {
		name  string
		data  string
		from  string
		event config.EventServer
	}{
		{"AddPlayer", `[{"id":"a","color":"#FF0000"}]`, "a", config.AddPlayer},
		{"MovePlayer", `{"id":"b","head":{}}`, "b", config.MovePlayer},
		{"RemovePlayer", "", "c", config.RemovePlayer},
		{"RayInteraction", `{"ray":"hit"}`, "d", config.RayInteraction},
		{"ActionsHands", `{"hand":"left","action":"grab"}`, "e", config.ActionHandsPlayer},
		{"DataConComillas", `{"key":"val\"ue"}`, "f", config.MovePlayer},
		{"FromVacio", "{}", "", config.AddPlayer},
		{"Unicode", `{"emoji":"🎮"}`, "g", config.RayInteraction},
		{"Event0", "", "h", config.RayInteraction},
		{"Event6", "{}", "i", config.IdentifyPlayer},
		{"ControlCharNUL", "hello\x00world", "j", config.MovePlayer},
		{"ControlCharSOH", "hello\x01world", "k", config.RayInteraction},
		{"ControlCharUS", "hello\x1Fworld", "l", config.ActionHandsPlayer},
		{"ControlCharNewline", "hello\nworld", "m", config.MovePlayer},
		{"TodosLosControlChars", "\x00\x01\x02\x03\x04\x05\x06\x07\x08\x09\x0A\x0B\x0C\x0D\x0E\x0F\x10\x11\x12\x13\x14\x15\x16\x17\x18\x19\x1A\x1B\x1C\x1D\x1E\x1F", "n", config.AddPlayer},
		{"FromConControlChar", "data", "evil\x00name", config.RayInteraction},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			wire, err := BuildWire(tc.data, tc.from, tc.event)
			if err != nil {
				t.Fatalf("BuildWire: %v", err)
			}

			var roundtrip typegame.MessageObject
			if err := json.Unmarshal(wire, &roundtrip); err != nil {
				t.Fatalf("OUTPUT NO ES JSON VALIDO: %s\n error: %v", wire, err)
			}

			if roundtrip.Data != tc.data {
				t.Fatalf("data roundtrip mismatch: got %q, want %q", roundtrip.Data, tc.data)
			}
			if roundtrip.From != tc.from {
				t.Fatalf("from roundtrip mismatch: got %q, want %q", roundtrip.From, tc.from)
			}
			if roundtrip.Event != tc.event {
				t.Fatalf("event roundtrip mismatch: got %d, want %d", roundtrip.Event, tc.event)
			}
		})
	}
}

func TestBuildWire_EstructuraCorrecta(t *testing.T) {
	out, err := BuildWire("hello", "world", config.AddPlayer)
	if err != nil {
		t.Fatal(err)
	}
	got := string(out)
	want := `{"data":"hello","from":"world","events":3}`
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestBuildWire_NoContieneControlCharsCrudos(t *testing.T) {
	wire, err := BuildWire("a\x00b\x01c\x1Fd", "x", config.MovePlayer)
	if err != nil {
		t.Fatal(err)
	}
	for i, b := range wire {
		if b < 0x20 && b != '\t' && b != '\n' && b != '\r' {
			t.Fatalf("byte crudo < 0x20 (no escapado) en posición %d: 0x%02x\n wire: %s", i, b, wire)
		}
	}
	if strings.ContainsRune(string(wire), '\x00') {
		t.Fatal("NUL byte presente sin escapar")
	}
}
