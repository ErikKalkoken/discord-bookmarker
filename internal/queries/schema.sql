CREATE TABLE IF NOT EXISTS bookmarks (
  id INTEGER PRIMARY KEY,
  author_id TEXT NOT NULL,
  channel_id TEXT NOT NULL,
  content TEXT NOT NULL,
  created_at DATETIME NOT NULL,
  due_at DATETIME,
  guild_id TEXT NOT NULL,
  message_id TEXT NOT NULL,
  timestamp DATETIME NOT NULL,
  updated_at DATETIME NOT NULL,
  user_id TEXT NOT NULL,
  UNIQUE (channel_id, guild_id, message_id, user_id)
);

CREATE INDEX IF NOT EXISTS reminders_idx_1 ON bookmarks (user_id);

CREATE INDEX IF NOT EXISTS reminders_idx_2 ON bookmarks (due_at);