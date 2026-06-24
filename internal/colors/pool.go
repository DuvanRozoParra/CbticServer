package colors

import (
	"fmt"
	"math"
	"sync"
)

const (
	hexFormat       = "#%02X%02X%02X"
	goldenAngle     = 137.5077640500378
	defaultSat      = 0.65
	defaultLight    = 0.55
	hueLayerSize    = 360.0
	lightLayerCount = 4.0
	satLayerCount   = 3.0
	lightLayerStep  = 0.12
	satLayerStep    = 0.15
	maxAcquireTries = 10000
)

type Pool struct {
	mu    sync.Mutex
	inUse map[string]struct{}
}

func NewPool() *Pool {
	return &Pool{
		inUse: make(map[string]struct{}),
	}
}

func (p *Pool) Acquire() (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for tries := 0; tries < maxAcquireTries; tries++ {
		h := math.Mod(float64(tries)*goldenAngle, 360)
		lLayer := math.Mod(float64(tries)/hueLayerSize, lightLayerCount)
		sLayer := math.Mod(float64(tries)/(hueLayerSize*lightLayerCount), satLayerCount)
		l := clamp01(defaultLight + (lLayer-(lightLayerCount-1)/2.0)*lightLayerStep)
		s := clamp01(defaultSat + (sLayer-(satLayerCount-1)/2.0)*satLayerStep)
		color := hslToHex(h, s, l)
		if _, taken := p.inUse[color]; !taken {
			p.inUse[color] = struct{}{}
			return color, nil
		}
	}
	return "", fmt.Errorf("pool de colores exhausto (límite teórico alcanzado)")
}

func (p *Pool) Release(hex string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.inUse, hex)
}

func hslToHex(h, s, l float64) string {
	c := (1 - math.Abs(2*l-1)) * s
	x := c * (1 - math.Abs(math.Mod(h/60, 2)-1))
	m := l - c/2

	var r, g, b float64
	switch {
	case h < 60:
		r, g, b = c, x, 0
	case h < 120:
		r, g, b = x, c, 0
	case h < 180:
		r, g, b = 0, c, x
	case h < 240:
		r, g, b = 0, x, c
	case h < 300:
		r, g, b = x, 0, c
	default:
		r, g, b = c, 0, x
	}

	R := int(math.Round((r + m) * 255))
	G := int(math.Round((g + m) * 255))
	B := int(math.Round((b + m) * 255))
	return fmt.Sprintf(hexFormat, clamp(R), clamp(G), clamp(B))
}

func clamp(v int) int {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return v
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
