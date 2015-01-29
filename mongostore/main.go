package mongostore

import (
	"net/http"
	"time"

	nSessions "github.com/goincremental/negroni-sessions"
	"github.com/gorilla/securecookie"
	gSessions "github.com/gorilla/sessions"
	"labix.org/v2/mgo"
	"labix.org/v2/mgo/bson"
)

// New returns a new mongo store
func New(session mgo.Session, database string, collection string, maxAge int, ensureTTL bool, keyPairs ...[]byte) nSessions.Store {

	if ensureTTL {
		conn := session.Clone()
		defer conn.Close()
		db := conn.DB(database)
		c := db.C(collection)
		c.EnsureIndex(mgo.Index{
			Key:         []string{"modified"},
			Background:  true,
			Sparse:      true,
			ExpireAfter: time.Duration(maxAge) * time.Second,
		})
	}
	return &mongoStore{
		Codecs:     securecookie.CodecsFromPairs(keyPairs...),
		Token:      nSessions.NewCookieToken(),
		session:    session,
		database:   database,
		collection: collection,
		options: &gSessions.Options{
			MaxAge: maxAge,
		},
	}
}

func (m *mongoStore) Options(options nSessions.Options) {
	m.options = &gSessions.Options{
		Path:     options.Path,
		Domain:   options.Domain,
		MaxAge:   options.MaxAge,
		Secure:   options.Secure,
		HttpOnly: options.HTTPOnly,
	}
}

type mongoSession struct {
	ID       bson.ObjectId `bson:"_id,omitempty"`
	Data     string
	Modified time.Time
}

type mongoStore struct {
	Codecs     []securecookie.Codec
	Token      nSessions.TokenGetSetter
	session    mgo.Session
	database   string
	collection string
	options    *gSessions.Options
}

//Implementation of gorilla/sessions.Store interface
// Get registers and returns a session for the given name and session store.
// It returns a new session if there are no sessions registered for the name.
func (m *mongoStore) Get(r *http.Request, name string) (*gSessions.Session, error) {
	return gSessions.GetRegistry(r).Get(m, name)
}

// New returns a session for the given name without adding it to the registry.
func (m *mongoStore) New(r *http.Request, name string) (*gSessions.Session, error) {
	session := gSessions.NewSession(m, name)
	session.Options = &gSessions.Options{
		Path:   m.options.Path,
		MaxAge: m.options.MaxAge,
	}
	session.IsNew = true
	var err error
	if cook, errToken := m.Token.GetToken(r, name); errToken == nil {
		err = securecookie.DecodeMulti(name, cook, &session.ID, m.Codecs...)
		if err == nil {
			ok, err := m.load(session)
			session.IsNew = !(err == nil && ok) // not new if no error and data available
		}
	}
	return session, err
}

func (m *mongoStore) Save(r *http.Request, w http.ResponseWriter, session *gSessions.Session) error {
	if session.Options.MaxAge < 0 {
		if err := m.delete(session); err != nil {
			return err
		}
		m.Token.SetToken(w, session.Name(), "", session.Options)
		return nil
	}

	if session.ID == "" {
		session.ID = bson.NewObjectId().Hex()
	}

	if err := m.save(session); err != nil {
		return err
	}

	encoded, err := securecookie.EncodeMulti(session.Name(), session.ID,
		m.Codecs...)
	if err != nil {
		return err
	}

	m.Token.SetToken(w, session.Name(), encoded, session.Options)
	return nil
}

func (m *mongoStore) load(session *gSessions.Session) (bool, error) {
	if !bson.IsObjectIdHex(session.ID) {
		return false, nSessions.ErrInvalidId
	}

	connection := m.session.Clone()
	defer connection.Close()
	db := connection.DB(m.database)
	c := db.C(m.collection)

	s := mongoSession{}
	err := c.FindId(bson.ObjectIdHex(session.ID)).One(&s)
	if err != nil {
		return false, err
	}

	if err := securecookie.DecodeMulti(session.Name(), s.Data, &session.Values,
		m.Codecs...); err != nil {
		return false, err
	}

	return true, nil
}

func (m *mongoStore) save(session *gSessions.Session) error {
	if !bson.IsObjectIdHex(session.ID) {
		return nSessions.ErrInvalidId
	}

	var modified time.Time
	if val, ok := session.Values["modified"]; ok {
		modified, ok = val.(time.Time)
		if !ok {
			return nSessions.ErrInvalidModified
		}
	} else {
		modified = time.Now()
	}

	encoded, err := securecookie.EncodeMulti(session.Name(), session.Values,
		m.Codecs...)
	if err != nil {
		return err
	}

	s := mongoSession{
		Data:     encoded,
		Modified: modified,
	}

	connection := m.session.Clone()
	defer connection.Close()
	db := connection.DB(m.database)
	c := db.C(m.collection)

	_, err = c.UpsertId(bson.ObjectIdHex(session.ID), &s)
	if err != nil {
		return err
	}

	return nil
}

func (m *mongoStore) delete(session *gSessions.Session) error {
	if !bson.IsObjectIdHex(session.ID) {
		return nSessions.ErrInvalidId
	}
	connection := m.session.Clone()
	defer connection.Close()
	db := connection.DB(m.database)
	c := db.C(m.collection)
	return c.RemoveId(bson.ObjectIdHex(session.ID))
}
