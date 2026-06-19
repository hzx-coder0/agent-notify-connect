package benchmark

import (
	"sync"
	"time"
)

// Context provides lightweight timing instrumentation for hook execution.
// Enabled via config.json: { "debug": { "benchmark": true } }.
// Output goes to the plugin log file via the provided logFunc.
type Context struct {
	mu      sync.Mutex
	events  []TimingEvent
	starts  map[string]time.Time
	enabled bool
	logFunc func(format string, args ...interface{})
}

// TimingEvent represents a single timing measurement.
type TimingEvent struct {
	Name     string  `json:"name"`
	Type     string  `json:"type"` // "interval", "checkpoint", "async"
	Duration float64 `json:"duration_ms"`
	Success  bool    `json:"success"`
}

// New creates a new benchmark context.
// When enabled, timing data is written to the log file via logFunc.
func New(enabled bool, logFunc func(string, ...interface{})) *Context {
	return &Context{
		starts:  make(map[string]time.Time),
		enabled: enabled,
		logFunc: logFunc,
	}
}

// Enabled returns whether benchmarking is active.
func (c *Context) Enabled() bool {
	return c.enabled
}

// Start begins timing a named operation.
func (c *Context) Start(name string) {
	if !c.enabled {
		return
	}
	c.mu.Lock()
	c.starts[name] = time.Now()
	c.mu.Unlock()
}

// Elapsed records the duration since the matching Start call.
func (c *Context) Elapsed(name string) {
	if !c.enabled {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	start, ok := c.starts[name]
	if !ok {
		return
	}
	c.events = append(c.events, TimingEvent{
		Name:     name,
		Type:     "interval",
		Duration: float64(time.Since(start).Microseconds()) / 1000.0,
		Success:  true,
	})
	delete(c.starts, name)
}

// Checkpoint records a point-in-time event with a success/failure flag.
func (c *Context) Checkpoint(name string, success bool) {
	if !c.enabled {
		return
	}
	c.mu.Lock()
	c.events = append(c.events, TimingEvent{
		Name:    name,
		Type:    "checkpoint",
		Success: success,
	})
	c.mu.Unlock()
}

// Async marks a non-blocking operation that runs in the background.
func (c *Context) Async(name string) {
	if !c.enabled {
		return
	}
	c.mu.Lock()
	c.events = append(c.events, TimingEvent{
		Name: name,
		Type: "async",
	})
	c.mu.Unlock()
}

// Report outputs all collected timing data to the plugin log file.
func (c *Context) Report() {
	if !c.enabled || len(c.events) == 0 || c.logFunc == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	c.logFunc("BENCH_TIMING:")
	for _, evt := range c.events {
		switch evt.Type {
		case "interval":
			c.logFunc("  %-10s %-30s %10.2fms  %v",
				evt.Type, evt.Name, evt.Duration, evt.Success)
		case "checkpoint":
			c.logFunc("  %-10s %-30s %10s    %v",
				evt.Type, evt.Name, "", evt.Success)
		case "async":
			c.logFunc("  %-10s %-30s",
				evt.Type, evt.Name)
		}
	}
}
