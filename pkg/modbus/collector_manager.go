package modbus

import (
	"fmt"
	"sync"

	"mbsscaner/config"
	"mbsscaner/pkg/http"
)

type CollectorManager struct {
	mu         sync.RWMutex
	collectors map[string]*Collector
	points     map[string]config.Point
	deviceMap  map[string]string // pointName -> deviceName
}

var collectorManager *CollectorManager

func InitCollectorManager() {
	collectorManager = &CollectorManager{
		collectors: make(map[string]*Collector),
		points:     make(map[string]config.Point),
		deviceMap:  make(map[string]string),
	}
	http.RegisterPointWriter(collectorManager)
}

func GetCollectorManager() *CollectorManager {
	return collectorManager
}

func (m *CollectorManager) RegisterCollector(deviceName string, collector *Collector) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.collectors[deviceName] = collector
}

func (m *CollectorManager) RegisterPoint(point config.Point, deviceName string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.points[point.Name] = point
	m.deviceMap[point.Name] = deviceName
}

func (m *CollectorManager) WritePoint(name string, value interface{}) error {
	m.mu.RLock()
	deviceName, ok := m.deviceMap[name]
	if !ok {
		m.mu.RUnlock()
		return fmt.Errorf("point not found: %s", name)
	}

	collector, ok := m.collectors[deviceName]
	if !ok {
		m.mu.RUnlock()
		return fmt.Errorf("collector not found for device: %s", deviceName)
	}

	point, ok := m.points[name]
	m.mu.RUnlock()

	if !ok {
		return fmt.Errorf("point config not found: %s", name)
	}

	return collector.WritePoint(point, value)
}

func (m *CollectorManager) GetPointValue(name string) (interface{}, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	deviceName, ok := m.deviceMap[name]
	if !ok {
		return nil, fmt.Errorf("point not found: %s", name)
	}

	collector, ok := m.collectors[deviceName]
	if !ok {
		return nil, fmt.Errorf("collector not found for device: %s", deviceName)
	}

	point, ok := m.points[name]
	if !ok {
		return nil, fmt.Errorf("point config not found: %s", name)
	}

	value, err := collector.ReadSinglePoint(point)
	if err != nil {
		return nil, err
	}

	return value.Value, nil
}
