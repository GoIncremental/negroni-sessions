// Package sessions contains middleware for easy session management in Negroni.
// Based on github.com/martini-contrib/sessions
//
//	package main
//
//	import (
//		"github.com/urfave/negroni"
//		"github.com/goincremental/negroni-sessions"
//		"net/http"
//	)
//
//	func main() {
//	n := negroni.Classic()
//
//		store := sessions.NewCookieStore([]byte("secret123"))
//		n.Use(sessions.Sessions("my_session", store))
//
//		mux := http.NewServeMux()
//		mux.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
//			session := sessions.GetSession(req)
//			session.Set("hello", "world")
//		})
//	}
package sessions

import (
	"context"
	"log"
	"net/http"

	gContext "github.com/gorilla/context"
	"github.com/gorilla/sessions"
	"github.com/urfave/negroni"
)

type contextKey int

const (
	errorFormat string     = "[sessions] ERROR! %s\n"
	sessionKey  contextKey = 0
)

// Store is an interface for custom session stores.
type Store interface {
	sessions.Store
	Options(Options)
}

// Options stores configuration for a session or session store.
//
// Fields are a subset of http.Cookie fields.
type Options struct {
	Path   string
	Domain string
	// MaxAge=0 means no 'Max-Age' attribute specified.
	// MaxAge<0 means delete cookie now, equivalently 'Max-Age: 0'.
	// MaxAge>0 means Max-Age attribute present and given in seconds.
	MaxAge   int
	Secure   bool
	HTTPOnly bool
}

// Session stores the values and optional configuration for a session.
type Session interface {
	// Get returns the session value associated to the given key.
	Get(key interface{}) interface{}
	// Set sets the session value associated to the given key.
	Set(key interface{}, val interface{})
	// Delete removes the session value associated to the given key.
	Delete(key interface{})
	// Clear deletes all values in the session.
	Clear()
	// AddFlash adds a flash message to the session.
	// A single variadic argument is accepted, and it is optional: it defines the flash key.
	// If not defined "_flash" is used by default.
	AddFlash(value interface{}, vars ...string)
	// Flashes returns a slice of flash messages from the session.
	// A single variadic argument is accepted, and it is optional: it defines the flash key.
	// If not defined "_flash" is used by default.
	Flashes(vars ...string) []interface{}
	// Options sets confuguration for a session.
	Options(Options)
}

// Sessions is a Middleware that maps a session.Session service into the negroni handler chain.
// Sessions can use a number of storage solutions with the given store.
func Sessions(name string, store Store) negroni.HandlerFunc {
	return func(res http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
		// Map to the Session interface
		s := &session{name, r, store, nil, false}

		// Add our session to the context we got from our request
		ctx := context.WithValue(r.Context(), sessionKey, s)

		// Use before hook to save out the session
		rw := res.(negroni.ResponseWriter)
		rw.Before(func(negroni.ResponseWriter) {
			if s.Written() {
				check(s.Session().Save(r, res))
			}
		})

		// Wrap our request with the new context
		r = r.WithContext(ctx)

		next(rw, r)
	}
}

type session struct {
	name    string
	request *http.Request
	store   Store
	session *sessions.Session
	written bool
}

// GetSession returns the session stored in the request context
func GetSession(req *http.Request) Session {
	if s, ok := req.Context().Value(sessionKey).(*session); ok {
		return s
	}
	return nil
}

func (s *session) Get(key interface{}) interface{} {
	sess := s.Session()
	if sess == nil {
		return nil
	}
	return sess.Values[key]
}

func (s *session) Set(key interface{}, val interface{}) {
	sess := s.Session()
	if sess == nil {
		return
	}
	sess.Values[key] = val
	s.written = true
}

func (s *session) Delete(key interface{}) {
	sess := s.Session()
	if sess == nil {
		return
	}
	delete(sess.Values, key)
	s.written = true
}

func (s *session) Clear() {
	sess := s.Session()
	if sess == nil {
		return
	}
	sess.Values = nil
	s.written = true
	gContext.Clear(s.request)
}

func (s *session) AddFlash(value interface{}, vars ...string) {
	sess := s.Session()
	if sess == nil {
		return
	}
	sess.AddFlash(value, vars...)
	s.written = true
}

func (s *session) Flashes(vars ...string) []interface{} {
	sess := s.Session()
	if sess == nil {
		return []interface{}{}
	}
	s.written = true
	return sess.Flashes(vars...)
}

func (s *session) Options(options Options) {
	sess := s.Session()
	if sess == nil {
		return
	}
	sess.Options = &sessions.Options{
		Path:     options.Path,
		Domain:   options.Domain,
		MaxAge:   options.MaxAge,
		Secure:   options.Secure,
		HttpOnly: options.HTTPOnly,
	}
}

func (s *session) Session() *sessions.Session {
	if s.session == nil {
		var err error
		s.session, err = s.store.Get(s.request, s.name)
		check(err)
	}

	return s.session
}

func (s *session) Written() bool {
	return s.written
}

func check(err error) {
	if err != nil {
		log.Printf(errorFormat, err)
	}
}
