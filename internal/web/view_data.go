package web

import (
	"net/http"
	"net/url"

	"github.com/google/uuid"
	"github.com/gorilla/csrf"
	"github.com/willemschots/househunt/internal"
	"github.com/willemschots/househunt/internal/errorz"
)

type viewData struct {
	Version     string
	CSRFToken   string
	IsLoggedIn  bool
	UserID      uuid.UUID
	Flashes     []any
	InputForm   url.Values
	InputErrors errorz.InvalidInput
	Data        any
}

// prepViewData prepares the data that will be passed to the view.
// Should be called before the preWrite method, because the session could still be altered at this point.
func (s *Server) prepViewData(r *http.Request, w http.ResponseWriter, data any) *viewData {
	sess, err := sessionFromCtx(r.Context())
	if err != nil {
		s.deps.Logger.Error("failed to get session", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return nil
	}

	userID, loggedIn := sess.UserID()

	return &viewData{
		Version:     internal.BuildRevision,
		CSRFToken:   csrf.Token(r),
		IsLoggedIn:  loggedIn,
		UserID:      userID,
		Flashes:     sess.ConsumeFlashes(),
		InputForm:   r.Form,
		InputErrors: nil,
		Data:        data,
	}
}
