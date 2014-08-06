package sessions

import (
	"net/http"
	"time"

	"github.com/goincremental/dal"
	"github.com/gorilla/securecookie"
	"github.com/gorilla/sessions"
)

// NewDalStore is a factory function that returns a store object using the provided dal.Connection
func NewDalStore(connection dal.Connection, database string, collection string, maxAge int, ensureTTL bool, keyPairs ...[]byte) Store {
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
		Token:      &cookieToken{},
		connection: connection,
		database:   database,
		collection: collection,
		options: &sessions.Options{
			MaxAge: maxAge,
		},
	}
}

func (d *dalStore) Options(options Options) {
	d.options = &sessions.Options{
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
	Token      tokenGetSeter
	connection dal.Connection
	database   string
	collection string
	options    *sessions.Options
}

//Implementation of gorilla/sessions.Store interface
// Get registers and returns a session for the given name and session store.
// It returns a new session if there are no sessions registered for the name.
func (d *dalStore) Get(r *http.Request, name string) (*sessions.Session, error) {
	return sessions.GetRegistry(r).Get(d, name)
}

// New returns a session for the given name without adding it to the registry.
func (d *dalStore) New(r *http.Request, name string) (*sessions.Session, error) {
	var err error
	session := sessions.NewSession(d, name)
	options := *d.options
	session.Options = &options
	session.IsNew = true

	if cook, errToken := d.Token.getToken(r, name); errToken == nil {
		err = securecookie.DecodeMulti(name, cook, &session.ID, d.Codecs...)
		if err == nil {
			ok, err := d.load(session)
			session.IsNew = !(err == nil && ok) // not new if no error and data available
		}
	}
	return session, err
}

func (d *dalStore) Save(r *http.Request, w http.ResponseWriter, session *sessions.Session) error {
	if session.Options.MaxAge < 0 {
		if err := d.delete(session); err != nil {
			return err
		}
		d.Token.setToken(w, session.Name(), "", session.Options)
		return nil
	}
	if session.ID == "" {
		session.ID = dal.NewObjectId().Hex()
	}

	if err := d.save(session); err != nil {
		return err
	}
	//save just the id to the cookie, the rest will be saved in the dal store
	encoded, err := securecookie.EncodeMulti(session.Name(), session.ID, d.Codecs...)

	if err != nil {
		return err
	}

	d.Token.setToken(w, session.Name(), encoded, session.Options)
	return err
}

func (d *dalStore) load(session *sessions.Session) (bool, error) {
	if !dal.IsObjectIdHex(session.ID) {
		return false, ErrInvalidId
	}
	conn := d.connection.Clone()
	defer conn.Close()
	db := conn.DB(d.database)
	c := db.C(d.collection)

	s := dalSession{}
	err := c.FindID(dal.ObjectIdHex(session.ID)).One(&s)
	if err != nil {
		return false, err
	}
	if err := securecookie.DecodeMulti(session.Name(), s.Data, &session.Values, d.Codecs...); err != nil {
		return false, err
	}
	return true, nil
}

func (d *dalStore) save(session *sessions.Session) error {
	if !dal.IsObjectIdHex(session.ID) {
		return ErrInvalidId
	}

	conn := d.connection.Clone()
	defer conn.Close()
	db := conn.DB(d.database)
	c := db.C(d.collection)

	var modified time.Time
	if val, ok := session.Values["modified"]; ok {
		modified, ok = val.(time.Time)
		if !ok {
			return ErrInvalidModified
		}
	} else {
		modified = time.Now()
	}

	encoded, err := securecookie.EncodeMulti(session.Name(), session.Values, d.Codecs...)
	if err != nil {
		return err
	}

	s := dalSession{
		ID:       dal.ObjectIdHex(session.ID),
		Data:     encoded,
		Modified: modified,
	}
	_, err = c.UpsertID(dal.ObjectIdHex(session.ID), &s)
	if err != nil {
		return err
	}

	return nil
}

func (d *dalStore) delete(session *sessions.Session) error {
	if !dal.IsObjectIdHex(session.ID) {
		return ErrInvalidId
	}

	conn := d.connection.Clone()
	defer conn.Close()
	db := conn.DB(d.database)
	c := db.C(d.collection)

	return c.RemoveID(dal.ObjectIdHex(session.ID))
}
