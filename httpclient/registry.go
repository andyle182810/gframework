package httpclient

import (
	"fmt"
	"sync"
)

type Registry struct {
	clients     map[string]*Client
	mu          sync.RWMutex
	defaultOpts []Option
}

func NewRegistry(defaultOpts ...Option) *Registry {
	return &Registry{
		clients:     make(map[string]*Client),
		mu:          sync.RWMutex{},
		defaultOpts: defaultOpts,
	}
}

func (r *Registry) Register(name, baseURL string, opts ...Option) *Registry {
	r.mu.Lock()
	defer r.mu.Unlock()

	allOpts := make([]Option, 0, len(r.defaultOpts)+len(opts))
	allOpts = append(allOpts, r.defaultOpts...)
	allOpts = append(allOpts, opts...)

	r.clients[name] = New(baseURL, allOpts...)

	return r
}

func (r *Registry) Client(name string) *Client {
	r.mu.RLock()
	defer r.mu.RUnlock()

	client, ok := r.clients[name]
	if !ok {
		panic(fmt.Sprintf("httpclient: service %q not registered", name))
	}

	return client
}

func (r *Registry) MustClient(name string) *Client {
	return r.Client(name)
}

func (r *Registry) GetClient(name string) (*Client, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	client, ok := r.clients[name]

	return client, ok
}

func (r *Registry) Has(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	_, ok := r.clients[name]

	return ok
}

func (r *Registry) Unregister(name string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	_, ok := r.clients[name]
	if ok {
		delete(r.clients, name)
	}

	return ok
}

func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.clients))
	for name := range r.clients {
		names = append(names, name)
	}

	return names
}

func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return len(r.clients)
}
