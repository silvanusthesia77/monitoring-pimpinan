package main

import "sync"

type SessionStore struct {
	mu       sync.RWMutex
	sessions map[string]User
}

func NewSessionStore() *SessionStore {
	return &SessionStore{sessions: map[string]User{}}
}

func (store *SessionStore) Create(user User) (string, error) {
	token, err := randomToken()
	if err != nil {
		return "", err
	}

	store.mu.Lock()
	store.sessions[token] = user
	store.mu.Unlock()
	return token, nil
}

func (store *SessionStore) Get(token string) (User, bool) {
	store.mu.RLock()
	user, ok := store.sessions[token]
	store.mu.RUnlock()
	return user, ok
}

func (store *SessionStore) Delete(token string) {
	store.mu.Lock()
	delete(store.sessions, token)
	store.mu.Unlock()
}
