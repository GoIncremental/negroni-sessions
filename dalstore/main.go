package dalstore

import (
	"net/http"
	"time"

	"github.com/goincremental/dal"
	nSessions "github.com/goincremental/negroni-sessions"
	"github.com/gorilla/securecookie"
	gSessions "github.com/gorilla/sessions"
)

// New is returns a store object using the provided dal.Connection
func New(connection dal.Connection, database string, collection string, maxAge int,
	ensureTTL bool, keyPairs ...[]byte) nSessions.Store {
	if ensureTTL {
		conn := connection.Clone()
		defer conn.Close()
		db := conn.DB(database)
		c := db.C(collection)
		c.EnsureIndex(dal.Index{
			Key:         []string{"modified"},
			Background:  true,
			Sparse:      true,
			ExpireAfter: time.Duration(maxAge) * time.Second,
		})
	}
	return &dalStore{
		Codecs:     securecookie.CodecsFromPairs(keyPairs...),
		Token:      nSessions.NewCookieToken(),
		connection: connection,
		database:   database,
		collection: collection,
		options: &gSessions.Options{
			MaxAge: maxAge,
		},
	}
}

func (d *dalStore) Options(options nSessions.Options) {
	d.options = &gSessions.Options{
		Path:     options.Path,
		Domain:   options.Domain,
		MaxAge:   options.MaxAge,
		Secure:   options.Secure,
		HttpOnly: options.HTTPOnly,
	}
}

type dalSession struct {
	ID       dal.ObjectID `bson:"_id,omitempty"`
	Data     string
	Modified time.Time
}

type dalStore struct {
	Codecs     []securecookie.Codec
	Token      nSessions.TokenGetSetter
	connection dal.Connection
	database   string
	collection string
	options    *gSessions.Options
}

//Implementation of gorilla/sessions.Store interface
// Get registers and returns a session for the given name and session store.
// It returns a new session if there are no sessions registered for the name.
func (d *dalStore) Get(r *http.Request, name string) (*gSessions.Session, error) {
	return gSessions.GetRegistry(r).Get(d, name)
}

// New returns a session for the given name without adding it to the registry.
func (d *dalStore) New(r *http.Request, name string) (*gSessions.Session, error) {
	var err error
	session := gSessions.NewSession(d, name)
	options := *d.options
	session.Options = &options
	session.IsNew = true

	if cook, errToken := d.Token.GetToken(r, name); errToken == nil {
		err = securecookie.DecodeMulti(name, cook, &session.ID, d.Codecs...)
		if err == nil {
			ok, err := d.load(session)
			session.IsNew = !(err == nil && ok) // not new if no error and data available
		}
	}
	return session, err
}

func (d *dalStore) Save(r *http.Request, w http.ResponseWriter, session *gSessions.Session) error {
	if session.Options.MaxAge < 0 {
		if err := d.delete(session); err != nil {
			return err
		}
		d.Token.SetToken(w, session.Name(), "", session.Options)
		return nil
	}
	if session.ID == "" {
		session.ID = dal.NewObjectID().Hex()
	}

	if err := d.save(session); err != nil {
		return err
	}
	//save just the id to the cookie, the rest will be saved in the dal store
	encoded, err := securecookie.EncodeMulti(session.Name(), session.ID, d.Codecs...)

	if err != nil {
		return err
	}

	d.Token.SetToken(w, session.Name(), encoded, session.Options)
	return err
}

func (d *dalStore) load(session *gSessions.Session) (bool, error) {
	if !dal.IsObjectIDHex(session.ID) {
		return false, nSessions.ErrInvalidId
	}
	conn := d.connection.Clone()
	defer conn.Close()
	db := conn.DB(d.database)
	c := db.C(d.collection)

	s := dalSession{}
	err := c.FindID(dal.ObjectIDHex(session.ID)).One(&s)
	if err != nil {
		return false, err
	}
	if err := securecookie.DecodeMulti(session.Name(), s.Data, &session.Values, d.Codecs...); err != nil {
		return false, err
	}
	return true, nil
}

func (d *dalStore) save(session *gSessions.Session) error {
	if !dal.IsObjectIDHex(session.ID) {
		return nSessions.ErrInvalidId
	}

	conn := d.connection.Clone()
	defer conn.Close()
	db := conn.DB(d.database)
	c := db.C(d.collection)

	var modified time.Time
	if val, ok := session.Values["modified"]; ok {
		modified, ok = val.(time.Time)
		if !ok {
			return nSessions.ErrInvalidModified
		}
	} else {
		modified = time.Now()
	}

	encoded, err := securecookie.EncodeMulti(session.Name(), session.Values, d.Codecs...)
	if err != nil {
		return err
	}

	s := dalSession{
		ID:       dal.ObjectIDHex(session.ID),
		Data:     encoded,
		Modified: modified,
	}
	_, err = c.UpsertID(dal.ObjectIDHex(session.ID), &s)
	if err != nil {
		return err
	}

	return nil
}

func (d *dalStore) delete(session *gSessions.Session) error {
	if !dal.IsObjectIDHex(session.ID) {
		return nSessions.ErrInvalidId
	}

	conn := d.connection.Clone()
	defer conn.Close()
	db := conn.DB(d.database)
	c := db.C(d.collection)

	return c.RemoveID(dal.ObjectIDHex(session.ID))
}
