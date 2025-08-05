package storage_test

import (
	"database/sql"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/icrowley/fake"
	"github.com/stretchr/testify/assert"

	"example/discord-bookmarker/internal/queries"
	"example/discord-bookmarker/internal/storage"
)

func TestBookmark(t *testing.T) {
	st := NewTestStorage(t)
	t.Run("can create new bookmark", func(t *testing.T) {
		ClearStorage(t, st)
		timestamp := time.Now()
		id, created, err := st.UpdateOrCreateBookmark(storage.UpdateOrCreateBookmarkParams{
			AuthorID:  "AuthorID",
			ChannelID: "ChannelID",
			Content:   "Content",
			GuildID:   "GuildID",
			MessageID: "MessageID",
			Timestamp: timestamp,
			UserID:    "UserID",
		})
		if assert.NoError(t, err) {
			assert.True(t, created)
			bm, err := st.GetBookmark(id)
			if assert.NoError(t, err) {
				assert.Equal(t, "AuthorID", bm.AuthorID)
				assert.Equal(t, "ChannelID", bm.ChannelID)
				assert.Equal(t, "Content", bm.Content)
				assert.Equal(t, "GuildID", bm.GuildID)
				assert.Equal(t, "MessageID", bm.MessageID)
				assert.True(t, timestamp.Equal(bm.Timestamp))
				assert.Equal(t, "UserID", bm.UserID)
			}
		}
	})
	t.Run("can update existing bookmark", func(t *testing.T) {
		ClearStorage(t, st)
		bm := CreateBookmark(t, st)
		dueAt := time.Now().Add(3 * time.Hour)
		id, created, err := st.UpdateOrCreateBookmark(storage.UpdateOrCreateBookmarkParams{
			ChannelID: bm.ChannelID,
			DueAt:     dueAt,
			GuildID:   bm.GuildID,
			MessageID: bm.MessageID,
			UserID:    bm.UserID,
			Timestamp: time.Now(),
		})
		if assert.NoError(t, err) {
			assert.False(t, created)
			bm, err := st.GetBookmark(id)
			if assert.NoError(t, err) {
				assert.True(t, bm.DueAt.Valid)
				assert.True(t, dueAt.Equal(bm.DueAt.Time))
			}
		}
	})
	t.Run("can list bookmarks for user", func(t *testing.T) {
		ClearStorage(t, st)
		userID := "abc123"
		bm1 := CreateBookmark(t, st, storage.UpdateOrCreateBookmarkParams{
			UserID: userID,
		})
		bm2 := CreateBookmark(t, st, storage.UpdateOrCreateBookmarkParams{
			UserID: userID,
		})
		CreateBookmark(t, st)
		xx, err := st.ListBookmarksForUser(userID)
		if assert.NoError(t, err) {
			want := []int64{bm1.ID, bm2.ID}
			var got []int64
			for _, x := range xx {
				got = append(got, x.ID)
			}
			assert.ElementsMatch(t, want, got)
		}
	})
	t.Run("can list due bookmarks", func(t *testing.T) {
		ClearStorage(t, st)
		now := time.Now().UTC()
		bm1 := CreateBookmark(t, st, storage.UpdateOrCreateBookmarkParams{
			DueAt: now.Add(-1 * time.Minute),
		})
		CreateBookmark(t, st, storage.UpdateOrCreateBookmarkParams{
			DueAt: now.Add(1 * time.Minute),
		})
		CreateBookmark(t, st)
		xx, err := st.ListDueBookmarks()
		if assert.NoError(t, err) {
			want := []int64{bm1.ID}
			var got []int64
			for _, x := range xx {
				got = append(got, x.ID)
			}
			assert.ElementsMatch(t, want, got)
		}
	})
	t.Run("can count bookmarks for user", func(t *testing.T) {
		ClearStorage(t, st)
		userID := "abc123"
		CreateBookmark(t, st, storage.UpdateOrCreateBookmarkParams{
			UserID: userID,
		})
		CreateBookmark(t, st, storage.UpdateOrCreateBookmarkParams{
			UserID: userID,
		})
		CreateBookmark(t, st)
		got, err := st.CountBookmarksForUser(userID)
		if assert.NoError(t, err) {
			assert.Equal(t, 2, got)
		}
	})
	t.Run("can delete bookmarks", func(t *testing.T) {
		ClearStorage(t, st)
		bm := CreateBookmark(t, st)
		err := st.DeleteBookmark(bm.ID)
		if assert.NoError(t, err) {
			_, err := st.GetBookmark(bm.ID)
			assert.ErrorIs(t, err, sql.ErrNoRows)
		}
	})
	t.Run("can set reminder", func(t *testing.T) {
		ClearStorage(t, st)
		bm := CreateBookmark(t, st)
		dueAt := time.Now().Add(3 * time.Hour)
		err := st.SetReminder(bm.ID, dueAt)
		if assert.NoError(t, err) {
			bm, err := st.GetBookmark(bm.ID)
			if assert.NoError(t, err) {
				assert.True(t, bm.DueAt.Valid)
				assert.True(t, dueAt.Equal(bm.DueAt.Time))
			}
		}
	})
	t.Run("can remove reminder", func(t *testing.T) {
		ClearStorage(t, st)
		bm := CreateBookmark(t, st, storage.UpdateOrCreateBookmarkParams{
			DueAt: time.Now().Add(3 * time.Hour),
		})
		err := st.RemoveReminder(bm.ID)
		if assert.NoError(t, err) {
			bm, err := st.GetBookmark(bm.ID)
			if assert.NoError(t, err) {
				assert.False(t, bm.DueAt.Valid)
			}
		}
	})
}

func NewTestStorage(t *testing.T) *storage.Storage {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(queries.DDL()); err != nil {
		t.Fatal(err)
	}
	return storage.New(db, db)
}

var (
	counterUniqueID atomic.Int64
)

func ClearStorage(t *testing.T, st *storage.Storage) {
	err := st.DeleteAllBookmarks()
	if err != nil {
		t.Fatal(err)
	}
}

func CreateBookmark(t *testing.T, st *storage.Storage, args ...storage.UpdateOrCreateBookmarkParams) queries.Bookmark {
	uniqueID := func() string {
		return "ID-" + strconv.Itoa(int(counterUniqueID.Add(1)))
	}
	var arg storage.UpdateOrCreateBookmarkParams
	if len(args) > 0 {
		arg = args[0]
	}
	if arg.AuthorID == "" {
		arg.AuthorID = uniqueID()
	}
	if arg.ChannelID == "" {
		arg.ChannelID = uniqueID()
	}
	if arg.Content == "" {
		arg.Content = fake.Paragraph()
	}
	if arg.GuildID == "" {
		arg.GuildID = uniqueID()
	}
	if arg.MessageID == "" {
		arg.MessageID = uniqueID()
	}
	if arg.Timestamp.IsZero() {
		arg.Timestamp = time.Now().UTC()
	}
	if arg.UserID == "" {
		arg.UserID = uniqueID()
	}
	id, _, err := st.UpdateOrCreateBookmark(arg)
	if err != nil {
		t.Fatal(err)
	}
	bm, err := st.GetBookmark(id)
	if err != nil {
		t.Fatal(err)
	}
	return bm
}
