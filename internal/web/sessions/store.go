package sessions

import (
	"net/http"

	"github.com/gorilla/sessions"
)

const CookieName = "hh-session"

type Store struct {
	store sessions.Store
}

func NewStore(store sessions.Store) *Store {
	return &Store{store: store}
}

func (s *Store) Get(r *http.Request) (*Session, error) {
	base, err := s.store.Get(r, CookieName)
	if err != nil {
		return nil, err
	}

	return &Session{base: base}, nil
}

func (s *Store) Save(r *http.Request, w http.ResponseWriter, sess *Session) error {
	err := s.store.Save(r, w, sess.base)
	if err != nil {
		return err
	}

	sess.needsSave = false
	return nil
}
