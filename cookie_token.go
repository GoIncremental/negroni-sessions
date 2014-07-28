package sessions

import (
	"github.com/gorilla/sessions"
	"net/http"
)

type tokenGetSeter interface {
	getToken(req *http.Request, name string) (string, error)
	setToken(rw http.ResponseWriter, name, value string, options *sessions.Options)
}

type cookieToken struct{}

func (c *cookieToken) getToken(req *http.Request, name string) (string, error) {
	cook, err := req.Cookie(name)
	if err != nil {
		return "", err
	}

	return cook.Value, nil
}

func (c *cookieToken) setToken(rw http.ResponseWriter, name string, value string,
	options *sessions.Options) {
	http.SetCookie(rw, sessions.NewCookie(name, value, options))
}
