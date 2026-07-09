package gate

import (
	"maps"
	"os"
	"strings"
	"sync"
)

// FeatureGate is a named boolean flag that controls whether a section of code
// or a page is visible at runtime.
type FeatureGate struct {
	Name        string
	Default     bool
	Description string
	Persist     bool
}

// GateRegistry stores the resolved boolean value for each named gate.
type GateRegistry struct {
	mu    sync.RWMutex
	defs  []FeatureGate
	vals  map[string]bool
	order []string
}

// NewGateRegistry creates an empty GateRegistry.
func NewGateRegistry() *GateRegistry {
	return &GateRegistry{
		vals: make(map[string]bool),
	}
}

// Register adds a gate definition and seeds its value from Default.
func (g *GateRegistry) Register(gate FeatureGate) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.register(gate)
}

func (g *GateRegistry) register(gate FeatureGate) {
	if _, exists := g.vals[gate.Name]; exists {
		panic("gate: duplicate gate name: " + gate.Name)
	}
	g.defs = append(g.defs, gate)
	g.vals[gate.Name] = gate.Default
	g.order = append(g.order, gate.Name)
}

// RegisterAll registers multiple gates in order.
func (g *GateRegistry) RegisterAll(gates []FeatureGate) {
	g.mu.Lock()
	defer g.mu.Unlock()
	for _, gate := range gates {
		g.register(gate)
	}
}

// Has reports whether a gate with the given name is registered. It lets
// framework code register built-in gates only when the app has not already
// defined them (Register panics on duplicates).
func (g *GateRegistry) Has(name string) bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	_, ok := g.vals[name]
	return ok
}

// Value returns the current boolean value of the named gate.
func (g *GateRegistry) Value(name string) bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	v, ok := g.vals[name]
	if !ok {
		return true // absent gate = enabled
	}
	return v
}

// Set sets a gate to the given value without persisting.
func (g *GateRegistry) Set(name string, value bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if _, ok := g.vals[name]; ok {
		g.vals[name] = value
	}
}

// Toggle flips a gate's current value.
func (g *GateRegistry) Toggle(name string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if v, ok := g.vals[name]; ok {
		g.vals[name] = !v
	}
}

// Defs returns the gate definitions in registration order.
func (g *GateRegistry) Defs() []FeatureGate {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.defs
}

// Order returns gate names in registration order.
func (g *GateRegistry) Order() []string {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.order
}

// Snapshot returns a copy of all gate values keyed by gate name.
func (g *GateRegistry) Snapshot() map[string]bool {
	g.mu.RLock()
	defer g.mu.RUnlock()
	out := make(map[string]bool, len(g.vals))
	maps.Copy(out, g.vals)
	return out
}

// ApplyMap overrides gate values from a map.
func (g *GateRegistry) ApplyMap(m map[string]bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	for k, v := range m {
		if _, ok := g.vals[k]; ok {
			g.vals[k] = v
		}
	}
}

// LoadFromEnv reads gate values from environment variables on startup.
func (g *GateRegistry) LoadFromEnv(appName string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	for _, gate := range g.defs {
		env := gateEnvKey(appName, gate.Name)
		v := os.Getenv(env)

		devEnv := gateDevEnvKey(appName, gate.Name)
		if devVal := os.Getenv(devEnv); devVal != "" {
			v = devVal
		}

		if v == "" {
			continue
		}
		switch strings.ToLower(v) {
		case "true", "1":
			g.vals[gate.Name] = true
		case "false", "0":
			g.vals[gate.Name] = false
		}
	}
}

// LoadFromEnvPrefix applies env-var overrides using a direct prefix.
func (g *GateRegistry) LoadFromEnvPrefix(prefix string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	for _, gate := range g.defs {
		suffix := strings.ToUpper(strings.Map(func(r rune) rune {
			if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
				return r
			}
			return '_'
		}, gate.Name))
		env := prefix + suffix
		v := os.Getenv(env)
		if v == "" {
			continue
		}
		switch strings.ToLower(v) {
		case "true", "1", "yes":
			g.vals[gate.Name] = true
		case "false", "0", "no":
			g.vals[gate.Name] = false
		}
	}
}

func gateEnvKey(appName, gateName string) string {
	parts := appName + "_GATE_" + gateName
	return strings.ToUpper(strings.Map(func(r rune) rune {
		if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			return r
		}
		return '_'
	}, parts))
}

func gateDevEnvKey(appName, gateName string) string {
	appNameClean := strings.ToUpper(strings.ReplaceAll(appName, " ", "_"))
	parts := appNameClean + "_DEV_" + gateName
	return strings.ToUpper(strings.Map(func(r rune) rune {
		if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			return r
		}
		return '_'
	}, parts))
}
