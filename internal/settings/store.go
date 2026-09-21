package settings

import "sync"

// Store is the in-process, mutex-guarded cache every shell-layer caller
// shares: loaded once, saved after every change.
type Store struct {
	mu   sync.Mutex
	path string
	s    Settings
}

// Open loads path into a new Store.
func Open(path string) (*Store, error) {
	s, err := Load(path)
	if err != nil {
		return nil, err
	}
	return &Store{path: path, s: s}, nil
}

// Get returns a copy of the current settings.
func (st *Store) Get() Settings {
	st.mu.Lock()
	defer st.mu.Unlock()
	return st.s
}

// Update mutates a copy under lock, then saves it.
func (st *Store) Update(fn func(*Settings)) error {
	st.mu.Lock()
	defer st.mu.Unlock()

	next := st.s
	fn(&next)
	if err := Save(st.path, next); err != nil {
		return err
	}
	st.s = next
	return nil
}
