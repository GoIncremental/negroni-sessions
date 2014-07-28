package sessions

import (
	"github.com/goincremental/dal"
	"github.com/gorilla/securecookie"
	"github.com/gorilla/sessions"
	"net/http"
	"time"
)

func NewDalStore(c dal.Collection, maxAge int, ensureTTL bool, keyPairs ...[]byte) Store {

	if ensureTTL {
		c.EnsureIndex(dal.Index{
			Key:         []string{"modified"},
			Background:  true,
			Sparse:      true,
			ExpireAfter: time.Duration(maxAge) * time.Second,
		})
	}
	return &dalStore{
		Codecs: securecookie.CodecsFromPairs(keyPairs...),
		Token:  &cookieToken{},
		coll:   c,
	}
}

func (d *dalStore) Options(options Options) {
	d.options = &sessions.Options{
		Path:     options.Path,
		Domain:   options.Domain,
		MaxAge:   options.MaxAge,
		Secure:   options.Secure,
		HttpOnly: options.HttpOnly,
	}
}

type dalSession struct {
	Id       dal.ObjectId `bson:"_id,omitempty"`
	Data     string
	Modified time.Time
}

type dalStore struct {
	Codecs  []securecookie.Codec
	Token   tokenGetSeter
	coll    dal.Collection
	options *sessions.Options
}

//Implementation of gorilla/sessions.Store interface
// Get registers and returns a session for the given name and session store.
// It returns a new session if there are no sessions registered for the name.
func (m *dalStore) Get(r *http.Request, name string) (*sessions.Session, error) {
	return sessions.GetRegistry(r).Get(m, name)
}

// New returns a session for the given name without adding it to the registry.
func (m *dalStore) New(r *http.Request, name string) (*sessions.Session, error) {
	session := sessions.NewSession(m, name)
	session.Options = &sessions.Options{
		Path:   m.options.Path,
		MaxAge: m.options.MaxAge,
	}
	session.IsNew = true
	var err error
	if cook, errToken := m.Token.getToken(r, name); errToken == nil {
		err = securecookie.DecodeMulti(name, cook, &session.ID, m.Codecs...)
		if err == nil {
			err = m.load(session)
			if err == nil {
				session.IsNew = false
			} else {
				err = nil
			}
		}
	}
	return session, err
}

func (m *dalStore) Save(r *http.Request, w http.ResponseWriter, session *sessions.Session) error {
	if session.Options.MaxAge < 0 {
		if err := m.delete(session); err != nil {
			return err
		}
		m.Token.setToken(w, session.Name(), "", session.Options)
		return nil
	}

	if session.ID == "" {
		session.ID = dal.NewObjectId().Hex()
	}

	if err := m.upsert(session); err != nil {
		return err
	}

	encoded, err := securecookie.EncodeMulti(session.Name(), session.ID,
		m.Codecs...)
	if err != nil {
		return err
	}

	m.Token.setToken(w, session.Name(), encoded, session.Options)
	return nil
}

func (m *dalStore) load(session *sessions.Session) error {
	if !dal.IsObjectIdHex(session.ID) {
		return ErrInvalidId
	}

	s := dalSession{}
	err := m.coll.FindId(dal.ObjectIdHex(session.ID)).One(&s)
	if err != nil {
		return err
	}

	if err := securecookie.DecodeMulti(session.Name(), s.Data, &session.Values,
		m.Codecs...); err != nil {
		return err
	}

	return nil
}

func (m *dalStore) upsert(session *sessions.Session) error {
	if !dal.IsObjectIdHex(session.ID) {
		return ErrInvalidId
	}

	var modified time.Time
	if val, ok := session.Values["modified"]; ok {
		modified, ok = val.(time.Time)
		if !ok {
			return ErrInvalidModified
		}
	} else {
		modified = time.Now()
	}

	encoded, err := securecookie.EncodeMulti(session.Name(), session.Values,
		m.Codecs...)
	if err != nil {
		return err
	}

	s := dalSession{
		Id:       dal.ObjectIdHex(session.ID),
		Data:     encoded,
		Modified: modified,
	}

	_, err = m.coll.UpsertId(s.Id, &s)
	if err != nil {
		return err
	}

	return nil
}

func (m *dalStore) delete(session *sessions.Session) error {
	if !dal.IsObjectIdHex(session.ID) {
		return ErrInvalidId
	}

	return m.coll.RemoveId(dal.ObjectIdHex(session.ID))
}
