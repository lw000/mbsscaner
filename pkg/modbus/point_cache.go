package modbus

import (
	"fmt"
	"sync"
	"time"

	"mbsscaner/pkg/http"
)

type PointCache struct {
	mu     sync.RWMutex
	points map[string]PointValue
}

var globalPointCache *PointCache

func InitPointCache() {
	globalPointCache = &PointCache{
		points: make(map[string]PointValue),
	}
	http.RegisterPointWriter(globalPointCache)
}

func (p *PointCache) WritePoint(name string, value interface{}) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.points[name] = PointValue{
		Name:      name,
		Value:     value,
		Timestamp: time.Now(),
		Quality:   "GOOD",
	}

	return nil
}

func (p *PointCache) GetPointValue(name string) (interface{}, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	point, ok := p.points[name]
	if !ok {
		return nil, fmt.Errorf("point not found: %s", name)
	}

	return point.Value, nil
}

func (p *PointCache) SetPointFromCollection(name string, value PointValue) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.points[name] = value
}

func (p *PointCache) GetPoint(name string) (PointValue, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	point, ok := p.points[name]
	return point, ok
}

func (p *PointCache) GetAllPoints() []PointValue {
	p.mu.RLock()
	defer p.mu.RUnlock()

	points := make([]PointValue, 0, len(p.points))
	for _, v := range p.points {
		points = append(points, v)
	}
	return points
}
