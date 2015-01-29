package sessions

import (
	"net/http"

	"github.com/gorilla/sessions"
)

//TokenGetSetter allows you to save and retrieve a value stored in a cookie
type TokenGetSetter interface {
	GetToken(req *http.Request, name string) (string, error)
	SetToken(rw http.ResponseWriter, name, value string, options *sessions.Options)
}

//NewCookieToken returns the default TokenGetSetter
func NewCookieToken() TokenGetSetter {
	return &cookieToken{}
}

type cookieToken struct{}

func (c *cookieToken) GetToken(req *http.Request, name string) (string, error) {
	cook, err := req.Cookie(name)
	if err != nil {
		return "", err
	}

	return cook.Value, nil
}

func (c *cookieToken) SetToken(rw http.ResponseWriter, name string, value string,
	options *sessions.Options) {
	http.SetCookie(rw, sessions.NewCookie(name, value, options))
}
