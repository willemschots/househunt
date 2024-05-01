package sessions

import (
	"github.com/google/uuid"
	"github.com/gorilla/sessions"
)

type Session struct {
	base      *sessions.Session
	needsSave bool
}

func (s *Session) NeedsSave() bool {
	return s.needsSave
}

func (s *Session) UserID() (uuid.UUID, bool) {
	userID, ok := s.base.Values["userID"].(uuid.UUID)
	return userID, ok
}

func (s *Session) SetUserID(userID uuid.UUID) {
	s.needsSave = true
	s.base.Values["userID"] = userID
}

func (s *Session) DeleteUserID() {
	s.needsSave = true
	delete(s.base.Values, "userID")
}

func (s *Session) AddFlash(flash any, vars ...string) {
	s.needsSave = true
	s.base.AddFlash(flash, vars...)
}

func (s *Session) Flashes() []any {
	return s.base.Flashes()
}
