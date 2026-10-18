package stats

import "botropolis.city/botropolis/pkg/claude"

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
