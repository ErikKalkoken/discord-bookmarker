-- name: CountBookmarks :one
SELECT
  COUNT(ID)
FROM
  bookmarks
WHERE
  user_id = ?;

-- name: GetBookmark :one
SELECT
  *
FROM
  bookmarks
WHERE
  id = ?
LIMIT
  1;

-- name: ListDueBookmarks :many
SELECT
  *
FROM
  bookmarks
WHERE
  due_at IS NOT NULL
  AND due_at < sqlc.arg(now);

-- name: ListBookmarksForUser :many
SELECT
  *
FROM
  bookmarks
WHERE
  user_id = ?
ORDER BY
  guild_id, channel_id, timestamp;

-- name: DeleteBookmark :exec
DELETE FROM bookmarks
WHERE
  id = ?;

-- name: DeleteAllBookmarks :exec
DELETE FROM bookmarks;

-- name: UpdateBookmarkDueAt :exec
Update bookmarks
SET
  due_at = ?
WHERE
  id = ?;

-- name: UpdateOrCreateBookmark :execlastid
INSERT INTO
  bookmarks (
    author_id,
    channel_id,
    content,
    created_at,
    due_at,
    guild_id,
    message_id,
    timestamp,
    updated_at,
    user_id
  )
VALUES
  (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (channel_id, guild_id, message_id, user_id) DO UPDATE
SET
  created_at = ?4,
  due_at = ?5,
  updated_at = ?9;
