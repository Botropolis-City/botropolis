#!/usr/bin/env bash
# Build stats.db fixtures at schema versions 1, 2 and 3 for item 149.
set -euo pipefail
if [[ "${1:-}" == "--help" || $# -ne 2 ]]; then
  echo "Usage: build-stats.sh SQL_DIR OUT_DIR — writes stats-1.db, stats-2.db, stats-3.db"
  exit 0
fi
sql=$1 out=$2
rm -f "$out"/stats-*.db
{ cat "$sql/schema-1.sql" "$sql/rows.sql"; echo "PRAGMA user_version = 1;"; } | sqlite3 "$out/stats-1.db"
cat "$sql/schema-1.sql" "$sql/rows.sql" "$sql/step-2.sql" | sqlite3 "$out/stats-2.db"
cat "$sql/schema-1.sql" "$sql/rows.sql" "$sql/step-2.sql" "$sql/step-3.sql" | sqlite3 "$out/stats-3.db"
for v in 1 2 3; do
  echo "stats-$v.db: user_version $(sqlite3 "$out/stats-$v.db" 'PRAGMA user_version'), $(sqlite3 "$out/stats-$v.db" 'SELECT count(*) FROM messages') message"
done
