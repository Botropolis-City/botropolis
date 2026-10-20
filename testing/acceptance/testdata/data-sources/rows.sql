INSERT INTO dirs (id, path) VALUES (1, '/w/lib');
INSERT INTO sessions (id, started_at, transcript_gone_at) VALUES ('aaaaaaaa-0000-0000-0000-000000000149', 1760950800, NULL);
INSERT INTO messages (id, at, model, speed, dir_id, thread, input, output, cache_create, cache_read)
  VALUES ('msg_0149', 1760950800, 'claude-opus-5', NULL, 1, 'main', 10, 20, 30, 40);
INSERT INTO message_sessions (message_id, session_id) VALUES ('msg_0149', 'aaaaaaaa-0000-0000-0000-000000000149');
INSERT INTO transcripts (path, session_id, size, mtime, read_to)
  VALUES ('/home/someone/.claude/projects/-w-lib/aaaaaaaa-0000-0000-0000-000000000149.jsonl', 'aaaaaaaa-0000-0000-0000-000000000149', 100, 1760950800, 100);
