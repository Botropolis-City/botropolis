package stats

// Forget deletes the rows of sessions prune removed: their links, their
// transcripts and the sessions themselves, then any message no remaining
// session holds and any directory no message uses. Item 110.
func (s *Store) Forget(sessions ...string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	for _, id := range sessions {
		for _, q := range []string{
			"DELETE FROM message_sessions WHERE session_id = ?",
			"DELETE FROM transcripts WHERE session_id = ?",
			"DELETE FROM sessions WHERE id = ?",
		} {
			if _, err := tx.Exec(q, id); err != nil {
				return err
			}
		}
	}
	for _, q := range []string{
		"DELETE FROM messages WHERE id NOT IN (SELECT message_id FROM message_sessions)",
		"DELETE FROM dirs WHERE id NOT IN (SELECT dir_id FROM messages)",
	} {
		if _, err := tx.Exec(q); err != nil {
			return err
		}
	}
	return tx.Commit()
}
