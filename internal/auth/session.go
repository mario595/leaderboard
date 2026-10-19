package auth

import (
	"database/sql"
	"net/http"
	"time"

	"github.com/alexedwards/scs/sqlite3store"
	"github.com/alexedwards/scs/v2"
)

// NewSessionManager returns a session manager that keeps sessions in the
// database, and a function that stops the store's background cleanup. The
// caller must call that function on shutdown.
func NewSessionManager(db *sql.DB, secureCookies bool) (*scs.SessionManager, func()) {
	st := sqlite3store.New(db)
	sm := scs.New()

	sm.Store = st
	sm.Lifetime = 30 * 24 * time.Hour
	sm.Cookie.Name = "session"
	sm.Cookie.HttpOnly = true
	sm.Cookie.SameSite = http.SameSiteLaxMode
	sm.Cookie.Secure = secureCookies
	sm.Cookie.Path = "/"

	return sm, st.StopCleanup
}
