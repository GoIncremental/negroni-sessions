package sessions_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/codegangsta/negroni"
	"github.com/goincremental/negroni-sessions"
	"github.com/goincremental/negroni-sessions/cookiestore"
)

func Test_Sessions(t *testing.T) {
	n := negroni.Classic()

	store := cookiestore.New([]byte("secret123"))
	n.Use(sessions.Sessions("my_session", store))

	mux := http.NewServeMux()
	mux.HandleFunc("/testsession", func(w http.ResponseWriter, req *http.Request) {
		session := sessions.GetSession(req)
		session.Set("hello", "world")
		fmt.Fprintf(w, "OK")
	})

	mux.HandleFunc("/show", func(w http.ResponseWriter, req *http.Request) {
		session := sessions.GetSession(req)
		if session.Get("hello") != "world" {
			t.Error("Session writing failed")
		}
	})

	n.UseHandler(mux)

	res := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/testsession", nil)
	n.ServeHTTP(res, req)

	res2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("GET", "/show", nil)
	req2.Header.Set("Cookie", res.Header().Get("Set-Cookie"))
	n.ServeHTTP(res2, req2)
}

func Test_SessionsDeleteValue(t *testing.T) {
	n := negroni.Classic()

	store := cookiestore.New([]byte("secret123"))
	n.Use(sessions.Sessions("my_session", store))

	mux := http.NewServeMux()

	mux.HandleFunc("/testsession", func(w http.ResponseWriter, req *http.Request) {
		session := sessions.GetSession(req)
		session.Set("hello", "world")
		session.Delete("hello")
		fmt.Fprintf(w, "OK")
	})

	mux.HandleFunc("/show", func(w http.ResponseWriter, req *http.Request) {
		session := sessions.GetSession(req)
		if session.Get("hello") == "world" {
			t.Error("Session value deleting failed")
		}
		fmt.Fprintf(w, "OK")
	})

	n.UseHandler(mux)

	res := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/testsession", nil)
	n.ServeHTTP(res, req)

	res2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("GET", "/show", nil)
	req2.Header.Set("Cookie", res.Header().Get("Set-Cookie"))
	n.ServeHTTP(res2, req2)
}

func Test_Options(t *testing.T) {
	n := negroni.Classic()
	store := cookiestore.New([]byte("secret123"))
	store.Options(sessions.Options{
		Domain: "negroni-sessions.goincremental.com",
	})

	n.Use(sessions.Sessions("my_session", store))

	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		session := sessions.GetSession(req)
		session.Set("hello", "world")
		session.Options(sessions.Options{
			Path: "/foo/bar/bat",
		})
		fmt.Fprintf(w, "OK")
	})

	mux.HandleFunc("/foo", func(w http.ResponseWriter, req *http.Request) {
		session := sessions.GetSession(req)
		session.Set("hello", "world")
		fmt.Fprintf(w, "OK")
	})

	n.UseHandler(mux)

	res := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/", nil)
	n.ServeHTTP(res, req)

	res2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("GET", "/foo", nil)
	n.ServeHTTP(res2, req2)

	s := strings.Split(res.Header().Get("Set-Cookie"), ";")
	if s[1] != " Path=/foo/bar/bat" {
		t.Error("Error writing path with options:", s[1])
	}

	s = strings.Split(res2.Header().Get("Set-Cookie"), ";")
	if s[1] != " Domain=negroni-sessions.goincremental.com" {
		t.Error("Error writing domain with options:", s[1])
	}
}

func Test_Flashes(t *testing.T) {
	n := negroni.Classic()

	store := cookiestore.New([]byte("secret123"))
	n.Use(sessions.Sessions("my_session", store))

	mux := http.NewServeMux()

	mux.HandleFunc("/set", func(w http.ResponseWriter, req *http.Request) {
		session := sessions.GetSession(req)
		session.AddFlash("hello world")
		fmt.Fprintf(w, "OK")
	})

	mux.HandleFunc("/show", func(w http.ResponseWriter, req *http.Request) {
		session := sessions.GetSession(req)
		l := len(session.Flashes())
		if l != 1 {
			t.Error("Flashes count does not equal 1. Equals ", l)
		}
		fmt.Fprintf(w, "OK")
	})

	mux.HandleFunc("/showagain", func(w http.ResponseWriter, req *http.Request) {
		session := sessions.GetSession(req)
		l := len(session.Flashes())
		if l != 0 {
			t.Error("flashes count is not 0 after reading. Equals ", l)
		}
		fmt.Fprintf(w, "OK")
	})

	n.UseHandler(mux)

	res := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/set", nil)
	n.ServeHTTP(res, req)

	res2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("GET", "/show", nil)
	req2.Header.Set("Cookie", res.Header().Get("Set-Cookie"))
	n.ServeHTTP(res2, req2)

	res3 := httptest.NewRecorder()
	req3, _ := http.NewRequest("GET", "/showagain", nil)
	req3.Header.Set("Cookie", res2.Header().Get("Set-Cookie"))
	n.ServeHTTP(res3, req3)
}

func Test_SessionsClear(t *testing.T) {
	n := negroni.Classic()
	data := map[string]string{
		"hello":  "world",
		"foo":    "bar",
		"apples": "oranges",
	}

	store := cookiestore.New([]byte("secret123"))
	n.Use(sessions.Sessions("my_session", store))

	mux := http.NewServeMux()

	mux.HandleFunc("/testsession", func(w http.ResponseWriter, req *http.Request) {
		session := sessions.GetSession(req)
		for k, v := range data {
			session.Set(k, v)
		}
		session.Clear()
		fmt.Fprintf(w, "OK")
	})

	mux.HandleFunc("/show", func(w http.ResponseWriter, req *http.Request) {
		session := sessions.GetSession(req)
		for k, v := range data {
			if session.Get(k) == v {
				t.Fatal("Session clear failed")
			}
		}
		fmt.Fprintf(w, "OK")
	})

	n.UseHandler(mux)

	res := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/testsession", nil)
	n.ServeHTTP(res, req)

	res2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("GET", "/show", nil)
	req2.Header.Set("Cookie", res.Header().Get("Set-Cookie"))
	n.ServeHTTP(res2, req2)
}
