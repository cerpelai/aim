package finder

import (
	"sort"
	"sync"

	"github.com/cerpelai/aim/internal/agent"
)

type Finder struct {
	providers []agent.Provider
}

// NewDefaultFinder creates a finder using all registered providers.
func NewDefaultFinder() *Finder {
	return &Finder{
		providers: agent.AllProviders(),
	}
}

// NewFinderWithProviders allows injecting custom or configured providers.
func NewFinderWithProviders(providers []agent.Provider) *Finder {
	return &Finder{
		providers: providers,
	}
}

func (f *Finder) FindAll(targetDir string, agentFilter string) ([]agent.Session, error) {
	var wg sync.WaitGroup
	var mu sync.Mutex
	var allSessions []agent.Session

	for _, p := range f.providers {
		if agentFilter != "" && p.Name() != agentFilter {
			continue
		}
		if !p.Available() {
			continue
		}

		wg.Add(1)
		go func(prov agent.Provider) {
			defer wg.Done()
			sessions, err := prov.FindSessions(targetDir)
			if err != nil {
				return
			}
			mu.Lock()
			allSessions = append(allSessions, sessions...)
			mu.Unlock()
		}(p)
	}

	wg.Wait()

	// Sort newest first
	sort.Slice(allSessions, func(i, j int) bool {
		return allSessions[i].LastUpdated.After(allSessions[j].LastUpdated)
	})

	return allSessions, nil
}

func (f *Finder) ProviderByName(name string) agent.Provider {
	for _, p := range f.providers {
		if p.Name() == name {
			return p
		}
	}
	return nil
}
