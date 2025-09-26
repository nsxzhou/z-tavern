package persona

// Store exposes persona retrieval for HTTP handlers.
type Store interface {
	List() []Persona
	FindByID(id string) (Persona, bool)
}

// MemoryStore implements Store with an in-memory slice, suitable for MVP.
type MemoryStore struct {
	items []Persona
}

// NewMemoryStore returns a MemoryStore preloaded with the supplied personas.
func NewMemoryStore(items []Persona) *MemoryStore {
	return &MemoryStore{items: append([]Persona(nil), items...)}
}

// List returns the predefined persona list.
func (s *MemoryStore) List() []Persona {
	return append([]Persona(nil), s.items...)
}

// FindByID looks up a persona by identifier.
func (s *MemoryStore) FindByID(id string) (Persona, bool) {
	for _, item := range s.items {
		if item.ID == id {
			return item, true
		}
	}
	return Persona{}, false
}
