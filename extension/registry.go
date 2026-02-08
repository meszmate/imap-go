package extension

import (
	"fmt"
	"sync"
)

// Registry manages extension registration and dependency resolution.
type Registry struct {
	mu         sync.RWMutex
	extensions map[string]Extension
	order      []string
}

// NewRegistry creates a new extension registry.
func NewRegistry() *Registry {
	return &Registry{
		extensions: make(map[string]Extension),
	}
}

// Register adds an extension to the registry.
// Returns an error if the extension is already registered.
func (r *Registry) Register(ext Extension) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := ext.Name()
	if _, exists := r.extensions[name]; exists {
		return fmt.Errorf("extension %q already registered", name)
	}

	r.extensions[name] = ext
	r.order = append(r.order, name)
	return nil
}

// Get returns a registered extension by name.
func (r *Registry) Get(name string) (Extension, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ext, ok := r.extensions[name]
	return ext, ok
}

// All returns all registered extensions in registration order.
func (r *Registry) All() []Extension {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]Extension, 0, len(r.order))
	for _, name := range r.order {
		result = append(result, r.extensions[name])
	}
	return result
}

// Resolve performs dependency resolution and returns extensions in
// topologically sorted order. Returns an error if there are missing
// dependencies or cycles.
func (r *Registry) Resolve() ([]Extension, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Check all dependencies exist
	for _, ext := range r.extensions {
		for _, dep := range ext.Dependencies() {
			if _, ok := r.extensions[dep]; !ok {
				return nil, fmt.Errorf("extension %q depends on %q which is not registered", ext.Name(), dep)
			}
		}
	}

	// Topological sort (Kahn's algorithm)
	inDegree := make(map[string]int)
	for name := range r.extensions {
		inDegree[name] = 0
	}
	for _, ext := range r.extensions {
		for _, dep := range ext.Dependencies() {
			inDegree[ext.Name()]++
			_ = dep // dep is depended upon, not the one with increased in-degree
		}
	}

	// Recompute properly: for each extension, its in-degree is the number of deps it has
	for name := range r.extensions {
		inDegree[name] = len(r.extensions[name].Dependencies())
	}

	var queue []string
	for name, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, name)
		}
	}

	var sorted []Extension
	for len(queue) > 0 {
		name := queue[0]
		queue = queue[1:]
		sorted = append(sorted, r.extensions[name])

		// For each extension that depends on this one, decrease in-degree
		for otherName, ext := range r.extensions {
			for _, dep := range ext.Dependencies() {
				if dep == name {
					inDegree[otherName]--
					if inDegree[otherName] == 0 {
						queue = append(queue, otherName)
					}
				}
			}
		}
	}

	if len(sorted) != len(r.extensions) {
		return nil, fmt.Errorf("circular dependency detected among extensions")
	}

	return sorted, nil
}

// Names returns the names of all registered extensions.
func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]string, len(r.order))
	copy(result, r.order)
	return result
}

// Len returns the number of registered extensions.
func (r *Registry) Len() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.extensions)
}

// Remove unregisters an extension by name.
func (r *Registry) Remove(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.extensions, name)
	for i, n := range r.order {
		if n == name {
			r.order = append(r.order[:i], r.order[i+1:]...)
			break
		}
	}
}
