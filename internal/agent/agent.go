package agent

import (
	"sync"
	"time"
)

// Session represents an AI agent session associated with a directory.
type Session struct {
	Agent       string    `json:"agent"`       // e.g. "claude", "agy", "gemini", "omp", "pi"
	ID          string    `json:"id"`          // Session ID / UUID
	Title       string    `json:"title"`       // Conversation title or first message preview
	Directory   string    `json:"directory"`   // Directory path this session is tied to
	LastUpdated time.Time `json:"lastUpdated"` // Last updated timestamp
	FilePath    string    `json:"filePath,omitempty"` // Path to session file / db (if applicable)
}

// Provider discovers sessions for a specific agent.
type Provider interface {
	Name() string
	Available() bool
	FindSessions(dir string) ([]Session, error)
	ResumeCommand(s Session) (name string, args []string, dir string)
}

// ConfigurableProvider allows providers to receive optional runtime config or overrides.
type ConfigurableProvider interface {
	Provider
	Configure(opts map[string]any) error
}

var (
	registryMu sync.RWMutex
	providers  = make(map[string]Provider)
	order      []string
)

// Register adds a provider to global registry.
func Register(p Provider) {
	registryMu.Lock()
	defer registryMu.Unlock()
	name := p.Name()
	if _, exists := providers[name]; !exists {
		order = append(order, name)
	}
	providers[name] = p
}

// AllProviders returns all registered providers in registration order.
func AllProviders() []Provider {
	registryMu.RLock()
	defer registryMu.RUnlock()
	list := make([]Provider, 0, len(order))
	for _, name := range order {
		list = append(list, providers[name])
	}
	return list
}

// GetProvider returns provider by name if registered.
func GetProvider(name string) (Provider, bool) {
	registryMu.RLock()
	defer registryMu.RUnlock()
	p, ok := providers[name]
	return p, ok
}
