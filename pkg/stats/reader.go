package stats

import (
	"database/sql"
	"errors"
	"os"
)

// The reasons a reader is paused rather than reading.
var (
	ErrNewer     = errors.New("this store is from a newer Botropolis — update to read it")
	ErrUpgrading = errors.New("the store is being upgraded")
	ErrNoStore   = errors.New("there are no statistics yet")
)

// OpenReader opens the store at path to read only: it never migrates,
// never records itself as a writer and cannot write, so the daemon stays
// the store's only writer. It refuses a store it cannot read with an
// error Reason puts in plain words. Item 116.
func OpenReader(path string) (*Store, error) {
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return nil, ErrNoStore
	}
	db, err := sql.Open("sqlite", "file:"+path+"?mode=ro&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, err
	}
	s := &Store{db: db}
	if err := s.readable(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) readable() error {
	var version int
	if err := s.db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		return err
	}
	switch {
	case version < Version:
		return ErrUpgrading
	case version > Version:
		var from int
		if err := s.db.QueryRow("SELECT value FROM meta WHERE key = 'readable_from'").Scan(&from); err != nil || from > Version {
			return ErrNewer
		}
	}
	return nil
}

// Reason is why statistics are paused, in plain words, or "" when err
// is nil.
func Reason(err error) string {
	switch {
	case err == nil:
		return ""
	case errors.Is(err, ErrNewer), errors.Is(err, ErrUpgrading), errors.Is(err, ErrNoStore):
		return err.Error()
	}
	return "the store could not be read: " + err.Error()
}
