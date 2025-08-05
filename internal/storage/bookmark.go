package storage

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"example/discord-bookmarker/internal/queries"
)

func (st *Storage) CountBookmarksForUser(userID string) (int, error) {
	x, err := st.qRO.CountBookmarks(context.Background(), userID)
	if err != nil {
		return 0, err
	}
	return int(x), nil
}

func (st *Storage) DeleteBookmark(id int64) error {
	err := st.qRW.DeleteBookmark(context.Background(), id)
	if err != nil {
		return fmt.Errorf("DeleteBookmark: ID %d: %w", id, err)
	}
	slog.Info("Bookmark deleted", "id", id)
	return nil
}

func (st *Storage) DeleteAllBookmarks() error {
	err := st.qRW.DeleteAllBookmarks(context.Background())
	if err != nil {
		return fmt.Errorf("DeleteAllBookmarks: %w", err)
	}
	slog.Info("All bookmark deleted")
	return nil
}

func (st *Storage) GetBookmark(id int64) (queries.Bookmark, error) {
	o, err := st.qRO.GetBookmark(context.Background(), id)
	if err != nil {
		return queries.Bookmark{}, err
	}
	return o, nil
}

func (st *Storage) RemoveReminder(id int64) error {
	err := st.qRW.UpdateBookmarkDueAt(context.Background(), queries.UpdateBookmarkDueAtParams{
		ID: id,
	})
	if err != nil {
		return fmt.Errorf("RemoveReminder: ID %d: %w", id, err)
	}
	slog.Info("Reminder removed", "id", id)
	return nil
}

func (st *Storage) SetReminder(id int64, dueAt time.Time) error {
	err := st.qRW.UpdateBookmarkDueAt(context.Background(), queries.UpdateBookmarkDueAtParams{
		ID:    id,
		DueAt: newNullTimeFromTime(dueAt),
	})
	if err != nil {
		return fmt.Errorf("SetReminder: ID %d: %w", id, err)
	}
	slog.Info("Reminder set", "id", id)
	return nil
}

func (st *Storage) ListBookmarksForUser(userID string) ([]queries.Bookmark, error) {
	return st.qRO.ListBookmarksForUser(context.Background(), userID)
}

func (st *Storage) ListDueBookmarks() ([]queries.Bookmark, error) {
	return st.qRO.ListDueBookmarks(context.Background(), newNullTimeFromTime(time.Now().UTC()))
}

type UpdateOrCreateBookmarkParams struct {
	AuthorID  string
	ChannelID string
	Content   string
	DueAt     time.Time
	GuildID   string
	MessageID string
	Timestamp time.Time
	UserID    string
}

func (arg UpdateOrCreateBookmarkParams) isValid() bool {
	if arg.ChannelID == "" || arg.MessageID == "" || arg.UserID == "" || arg.Timestamp.IsZero() {
		return false
	}
	return true
}

func (st *Storage) UpdateOrCreateBookmark(arg UpdateOrCreateBookmarkParams) (int64, bool, error) {
	wrapErr := func(err error) error {
		return fmt.Errorf("UpdateOrCreateBookmark: %+v: %w", arg, err)
	}
	if !arg.isValid() {
		return 0, false, wrapErr(fmt.Errorf("invalid arg"))
	}
	ctx := context.Background()
	tx, err := st.dbRW.Begin()
	if err != nil {
		return 0, false, wrapErr(err)
	}
	defer tx.Rollback()
	qtx := st.qRW.WithTx(tx)
	c1, err := qtx.CountBookmarks(ctx, arg.UserID)
	if err != nil {
		return 0, false, wrapErr(err)
	}
	id, err := qtx.UpdateOrCreateBookmark(ctx, queries.UpdateOrCreateBookmarkParams{
		AuthorID:  arg.AuthorID,
		ChannelID: arg.ChannelID,
		Content:   arg.Content,
		CreatedAt: time.Now().UTC(),
		DueAt:     newNullTimeFromTime(arg.DueAt),
		GuildID:   arg.GuildID,
		MessageID: arg.MessageID,
		Timestamp: arg.Timestamp,
		UpdatedAt: time.Now().UTC(),
		UserID:    arg.UserID,
	})
	if err != nil {
		return 0, false, wrapErr(err)
	}
	c2, err := qtx.CountBookmarks(ctx, arg.UserID)
	if err != nil {
		return 0, false, wrapErr(err)
	}
	if err := tx.Commit(); err != nil {
		return 0, false, wrapErr(err)
	}
	created := c2 > c1
	slog.Info("Updated bookmark", "id", id, "created", created, "user", arg.UserID)
	return id, created, nil
}

// newNullTimeFromTime returns a value as null type. Will assume not set when value is zero.
func newNullTimeFromTime(v time.Time) sql.NullTime {
	if v.IsZero() {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: v, Valid: true}
}
