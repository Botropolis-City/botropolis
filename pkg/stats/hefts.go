package stats

import (
	"database/sql"
	"errors"
	"sort"

	"botropolis.city/botropolis/pkg/claude"
)

// Hefts is every stored session's four counters, summed over every
// message it holds: a fork's copies and its subagents' messages count,
// and a session whose transcript is gone keeps its sum. The heft itself
// is computed from these by whoever reads them, so changing its formula
// never touches what is stored. Item 110.
func (s *Store) Hefts() (map[string]claude.Usage, error) {
	rows, err := s.db.Query(`SELECT ms.session_id, sum(m.input), sum(m.output), sum(m.cache_create), sum(m.cache_read), count(*)
		FROM message_sessions ms JOIN messages m ON m.id = ms.message_id
		GROUP BY ms.session_id`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	hefts := map[string]claude.Usage{}
	for rows.Next() {
		var id string
		var u claude.Usage
		if err := rows.Scan(&id, &u.Input, &u.Output, &u.CacheCreate, &u.CacheRead, &u.Messages); err != nil {
			return nil, err
		}
		hefts[id] = u
	}
	return hefts, rows.Err()
}

// MovedHefts is the four counters of every session whose rows changed
// since the last call, and which sessions those are: a moved session
// with no counters has been forgotten. Each is taken off the record as
// it is read, so a write after the read is reported next time. Item 196.
func (s *Store) MovedHefts() (map[string]claude.Usage, []string, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = tx.Rollback() }()
	rows, err := tx.Query("DELETE FROM moved RETURNING session_id")
	if err != nil {
		return nil, nil, err
	}
	var moved []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			_ = rows.Close()
			return nil, nil, err
		}
		moved = append(moved, id)
	}
	if err := rows.Close(); err != nil {
		return nil, nil, err
	}
	sort.Strings(moved)
	hefts := map[string]claude.Usage{}
	for _, id := range moved {
		var u claude.Usage
		err := tx.QueryRow(`SELECT sum(m.input), sum(m.output), sum(m.cache_create), sum(m.cache_read), count(*)
			FROM message_sessions ms JOIN messages m ON m.id = ms.message_id
			WHERE ms.session_id = ? GROUP BY ms.session_id`, id).Scan(&u.Input, &u.Output, &u.CacheCreate, &u.CacheRead, &u.Messages)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return nil, nil, err
		}
		hefts[id] = u
	}
	return hefts, moved, tx.Commit()
}
