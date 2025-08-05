// Package bot contains the Discord bot.
package bot

import (
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/docker/go-units"

	"example/discord-remindme/internal/queries"
	"example/discord-remindme/internal/storage"
)

const (
	formatDateTime = "2006-01-02 15:04"
	// ColorAqua              = 1752220  // #1ABC9C
	// ColorBlack             = 2303786  // #23272A
	// ColorBlue              = 3447003  // #3498DB
	// ColorBlurple           = 5793266  // #5865F2
	// ColorDarkAqua          = 1146986  // #11806A
	// ColorDarkBlue          = 2123412  // #206694
	// ColorDarkButNotBlack   = 2895667  // #2C2F33
	// ColorDarkerGrey        = 8359053  // #7F8C8D
	// ColorDarkGold          = 12745742 // #C27C0E
	// ColorDarkGreen         = 2067276  // #1F8B4C
	// ColorDarkGrey          = 9936031  // #979C9F
	// ColorDarkNavy          = 2899536  // #2C3E50
	// ColorDarkOrange        = 11027200 // #A84300
	// ColorDarkPurple        = 7419530  // #71368A
	// ColorDarkRed           = 10038562 // #992D22
	// ColorDarkVividPink     = 11342935 // #AD1457
	// ColorFuchsia           = 15418782 // #EB459E
	// ColorGold              = 15844367 // #F1C40F
	// ColorGreen             = 5763719  // #57F287
	// ColorGrey              = 9807270  // #95A5A6
	// ColorGreyple           = 10070709 // #99AAb5
	// ColorLightGrey         = 12370112 // #BCC0C0
	// ColorLuminousVividPink = 15277667 // #E91E63
	// ColorNavy              = 3426654  // #34495E
	// ColorNotQuiteBlack     = 2303786  // #23272A
	// ColorPurple            = 10181046 // #9B59B6
	// ColorRed               = 15548997 // #ED4245
	// ColorWhite             = 16777215 // #FFFFFF
	// colorYellow         = 16705372 // #FEE75C
	colorOrange         = 0xE67E22 // #E67E22
	maxBookmarksPerUser = 100
)

// Discord command names for interactions
const (
	// Create a bookmark for a message
	cmdCreateBookmark = "Bookmark"
	// Create a bookmark for a message with a reminder
	cmdCreateBookmarkWithReminder = "Bookmark With Reminder"
	// Bookmarker base command
	cmdBookmarkerBase = "bookmarker"
	// List bookmarks
	cmdListBookmarks = "list"
	// Remove bookmarks
	cmdRemoveBookmarks = "remove"
	// Set reminder for bookmark
	cmdRemindBookmarks = "remind"
	// Send a test DM to the user
	cmdTest = "test"
)

// Discord custom IDs for interactions
const (
	idCancelRemove   = "cancel-remove"
	idNewReminder    = "new-reminder"
	idRemoveBookmark = "remove-bookmark"
	idSetReminder    = "set-reminder"
)

// Discord commands
var commands = []discordgo.ApplicationCommand{
	{
		Name:        cmdBookmarkerBase,
		Description: "Manage bookmarks",
		Type:        discordgo.ChatApplicationCommand,
		IntegrationTypes: &[]discordgo.ApplicationIntegrationType{
			discordgo.ApplicationIntegrationUserInstall,
		},
		Contexts: &[]discordgo.InteractionContextType{
			discordgo.InteractionContextBotDM,
			discordgo.InteractionContextPrivateChannel,
			discordgo.InteractionContextGuild,
		},
		Options: []*discordgo.ApplicationCommandOption{
			{
				Description: "List bookmarks",
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        cmdListBookmarks,
			},
			{
				Description: "Remove bookmarks",
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        cmdRemoveBookmarks,
				Options: []*discordgo.ApplicationCommandOption{
					{
						Type:        discordgo.ApplicationCommandOptionInteger,
						Required:    true,
						Description: "Bookmark ID",
						Name:        "bookmark-id",
					},
				},
			},
			{
				Description: "Set or remove reminder for bookmarks",
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        cmdRemindBookmarks,
				Options: []*discordgo.ApplicationCommandOption{
					{
						Type:        discordgo.ApplicationCommandOptionInteger,
						Required:    true,
						Description: "Bookmark ID",
						Name:        "bookmark-id",
					},
				},
			},
			{
				Description: "Send test DM",
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        cmdTest,
			},
		},
	},
	{
		Name: cmdCreateBookmark,
		Type: discordgo.MessageApplicationCommand,
		IntegrationTypes: &[]discordgo.ApplicationIntegrationType{
			discordgo.ApplicationIntegrationUserInstall,
		},
		Contexts: &[]discordgo.InteractionContextType{
			discordgo.InteractionContextBotDM,
			discordgo.InteractionContextPrivateChannel,
			discordgo.InteractionContextGuild,
		},
	},
	{
		Name: cmdCreateBookmarkWithReminder,
		Type: discordgo.MessageApplicationCommand,
		IntegrationTypes: &[]discordgo.ApplicationIntegrationType{
			discordgo.ApplicationIntegrationUserInstall,
		},
		Contexts: &[]discordgo.InteractionContextType{
			discordgo.InteractionContextBotDM,
			discordgo.InteractionContextPrivateChannel,
			discordgo.InteractionContextGuild,
		},
	},
}

// discordMessage represents a Discord message.
type discordMessage struct {
	authorID  string
	channelID string
	content   string
	guildID   string
	messageID string
	timestamp time.Time
}

func (x discordMessage) UID() string {
	return messageUID(x.guildID, x.channelID, x.messageID)
}

func messageUID(guildID, channelID, messageID string) string {
	return fmt.Sprintf("%s-%s-%s", guildID, channelID, messageID)
}

type User struct {
	ID        string
	Name      string
	AvatarURL string
}

type Bot struct {
	appID string
	ds    *discordgo.Session
	st    *storage.Storage

	messageCache sync.Map
	userCache    sync.Map
}

// New registered a Discord bot with all interactions and returns it.
func New(st *storage.Storage, ds *discordgo.Session, appID string) *Bot {
	b := &Bot{
		appID: appID,
		st:    st,
		ds:    ds,
	}
	ds.AddHandler(func(s *discordgo.Session, r *discordgo.Ready) {
		slog.Info("Bot is up!")
	})
	ds.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		err := func() error {
			switch i.Type {
			case discordgo.InteractionApplicationCommand:
				return b.handleApplicationCommand(i)
			case discordgo.InteractionMessageComponent:
				return b.handleMessageComponent(i)
			}
			return fmt.Errorf("unexpected interaction type %d", i.Type)
		}()
		if err != nil {
			slog.Error("interaction failed", "error", err)
		}
	})
	return b
}

func (b *Bot) Start() {
	ticker := time.NewTicker(15 * time.Second)
	go func() {
		for {
			<-ticker.C
			bookmarks, err := b.st.ListDueBookmarks()
			if err != nil {
				slog.Error("Failed to fetch due bookmarks", "error", err)
				continue
			}
			if len(bookmarks) == 0 {
				continue
			}
			for _, r := range bookmarks {
				author, err := b.fetchUser(r.AuthorID)
				if err != nil {
					panic(err)
				}
				err = b.sendDM(
					r.UserID,
					fmt.Sprintf("You asked me to remind you about this message from %s:", author.Name),
					[]*discordgo.MessageEmbed{
						b.makeEmbedFromBookmark(r, makeEmbedFromBookmarkOpts{hideDue: true}),
					})
				if err != nil {
					slog.Error("Failed to send DM", "error", err)
					continue
				}
				slog.Info("Reminder sent", "user", r.UserID, "id", r.ID)
				if err := b.st.RemoveReminder(r.ID); err != nil {
					slog.Error("Failed to reset bookmark", "error", err)
					continue
				}
			}
		}
	}()
}

func (b *Bot) sendDM(userID string, content string, embeds []*discordgo.MessageEmbed) error {
	c, err := b.ds.UserChannelCreate(userID)
	if err != nil {
		return err
	}
	if _, err := b.ds.ChannelMessageSendComplex(c.ID, &discordgo.MessageSend{
		Content: content,
		Embeds:  embeds,
	}); err != nil {
		return err
	}
	return nil
}

func (b *Bot) ResetCommands() error {
	// Delete existing commands (if any)
	cc, err := b.ds.ApplicationCommands(b.appID, "")
	if err != nil {
		return err
	}
	for _, cmd := range cc {
		err := b.ds.ApplicationCommandDelete(b.appID, "", cmd.ID)
		if err != nil {
			return fmt.Errorf("delete application command %s: %w", cmd.Name, err)
		}
		slog.Info("Deleted application command", "cmd", cmd.Name)
	}
	// Add commands
	for _, cmd := range commands {
		_, err := b.ds.ApplicationCommandCreate(b.appID, "", &cmd)
		if err != nil {
			return fmt.Errorf("create application command %s: %w", cmd.Name, err)
		}
		slog.Info("Added application command", "cmd", cmd.Name)
	}
	return nil
}

func interactionUserID(i *discordgo.InteractionCreate) (string, error) {
	if i.Member != nil && i.Member.User != nil {
		return i.Member.User.ID, nil
	}
	if i.User != nil {
		return i.User.ID, nil
	}
	return "", fmt.Errorf("no user found for interaction")
}

func (b *Bot) handleApplicationCommand(i *discordgo.InteractionCreate) error {
	createMessageContext := func() discordMessage {
		data := i.ApplicationCommandData()
		messageID := data.TargetID
		message := data.Resolved.Messages[messageID]
		m := discordMessage{
			authorID:  message.Author.ID,
			channelID: message.ChannelID,
			content:   message.Content,
			guildID:   i.GuildID,
			messageID: messageID,
			timestamp: message.Timestamp,
		}
		return m
	}
	respondWithMessage := func(content string) error {
		err := b.ds.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: content,
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return err
	}
	responseWithReminderSelect := func(customID string, bm queries.Bookmark) error {
		err := b.ds.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "I will remind you about this message in...",
				Embeds: []*discordgo.MessageEmbed{
					b.makeEmbedFromBookmark(bm, makeEmbedFromBookmarkOpts{}),
				},
				Flags: discordgo.MessageFlagsEphemeral,
				Components: []discordgo.MessageComponent{
					discordgo.ActionsRow{
						Components: []discordgo.MessageComponent{
							discordgo.SelectMenu{
								CustomID:    customID,
								Placeholder: "Choose reminder duration",
								Options: []discordgo.SelectMenuOption{
									{
										Label: "10 seconds",
										Value: "10",
									},
									{
										Label: "1 hour",
										Value: "3600",
									},
									{
										Label: "3 hours",
										Value: "10800",
									},
									{
										Label: "1 day",
										Value: "86400",
									},
									{
										Label: "3 days",
										Value: "259200",
									},
									{
										Label: "1 week",
										Value: "604800",
									},
									{
										Label: "Never",
										Value: "0",
									},
								},
							},
						},
					},
				},
			},
		})
		return err
	}

	userID, err := interactionUserID(i)
	if err != nil {
		return err
	}
	data := i.ApplicationCommandData()
	name := data.Name
	switch name {
	case cmdCreateBookmark:
		total, err := b.st.CountBookmarksForUser(userID)
		if err != nil {
			return err
		}
		if total == maxBookmarksPerUser {
			return respondWithMessage(fmt.Sprintf(
				"You reached the maximum of %d bookmarks. Please remove bookmarks before adding new ones.",
				maxBookmarksPerUser,
			))
		}
		m := createMessageContext()
		id, created, err := b.st.UpdateOrCreateBookmark(storage.UpdateOrCreateBookmarkParams{
			AuthorID:  m.authorID,
			ChannelID: m.channelID,
			Content:   m.content,
			GuildID:   m.guildID,
			MessageID: m.messageID,
			Timestamp: m.timestamp,
			UserID:    userID,
		})
		if err != nil {
			return err
		}
		var s string
		if created {
			s = "created"
		} else {
			s = "updated"
		}
		return respondWithMessage(fmt.Sprintf("Bookmark #%d %s", id, s))

	case cmdCreateBookmarkWithReminder:
		m := createMessageContext()
		b.messageCache.Store(m.UID(), m)
		return responseWithReminderSelect(idNewReminder, queries.Bookmark{
			AuthorID:  m.authorID,
			ChannelID: m.channelID,
			Content:   m.content,
			GuildID:   m.guildID,
			MessageID: m.messageID,
			Timestamp: m.timestamp,
			UserID:    userID,
		})

	case cmdBookmarkerBase:
		if len(data.Options) == 0 {
			return fmt.Errorf("expected command options")
		}
		cmdOption := data.Options[0]
		switch cmdOption.Name {
		case cmdListBookmarks:
			bookmarks, err := b.st.ListBookmarksForUser(userID)
			if err != nil {
				return err
			}
			if len(bookmarks) == 0 {
				return respondWithMessage("No bookmarked messages yet")
			}
			err = b.ds.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Flags: discordgo.MessageFlagsEphemeral,
				},
			})
			if err != nil {
				return err
			}
			const maxBookmarksPerPage = 10
			pages := int(math.Ceil(float64(len(bookmarks)) / maxBookmarksPerPage))
			page := 1
			for chunk := range slices.Chunk(bookmarks, maxBookmarksPerPage) {
				content := fmt.Sprintf("%d bookmarked messages", len(bookmarks))
				if pages > 1 {
					content += fmt.Sprintf(" [%d/%d]", page, pages)
				}
				embeds := make([]*discordgo.MessageEmbed, 0)
				for _, bm := range chunk {
					embeds = append(embeds, b.makeEmbedFromBookmark(bm, makeEmbedFromBookmarkOpts{}))
				}
				_, err = b.ds.FollowupMessageCreate(i.Interaction, false, &discordgo.WebhookParams{
					Content: content,
					Embeds:  embeds,
					Flags:   discordgo.MessageFlagsEphemeral,
				})
				if err != nil {
					return err
				}
				page++
			}
			return nil

		case cmdRemoveBookmarks:
			if len(cmdOption.Options) != 1 {
				return fmt.Errorf("expected one option only: %+v", cmdOption.Options)
			}
			id := cmdOption.Options[0].IntValue()
			bm, err := b.st.GetBookmark(id)
			if errors.Is(err, sql.ErrNoRows) {
				return respondWithMessage(fmt.Sprintf("No bookmark found with ID #%d", id))
			} else if err != nil {
				return err
			}
			err = b.ds.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "Are you sure you want to remove this bookmark?",
					Flags:   discordgo.MessageFlagsEphemeral,
					Embeds: []*discordgo.MessageEmbed{
						b.makeEmbedFromBookmark(bm, makeEmbedFromBookmarkOpts{}),
					},
					Components: []discordgo.MessageComponent{
						discordgo.ActionsRow{
							Components: []discordgo.MessageComponent{
								discordgo.Button{
									Label:    "Remove",
									Style:    discordgo.DangerButton,
									CustomID: fmt.Sprintf("%s%d", idRemoveBookmark, id),
								},
								discordgo.Button{
									Label:    "Cancel",
									CustomID: idCancelRemove,
								},
							},
						},
					},
				},
			})
			return err

		case cmdRemindBookmarks:
			if len(cmdOption.Options) != 1 {
				return fmt.Errorf("expected one option only: %+v", cmdOption.Options)
			}
			id := cmdOption.Options[0].IntValue()
			bm, err := b.st.GetBookmark(id)
			if errors.Is(err, sql.ErrNoRows) {
				return respondWithMessage(fmt.Sprintf("No bookmark found with ID #%d", id))
			} else if err != nil {
				return err
			}
			return responseWithReminderSelect(fmt.Sprintf("%s%d", idSetReminder, bm.ID), bm)

		case cmdTest:
			err := b.sendDM(userID, "Hi, there! I am ready to assist you.", nil)
			if err != nil {
				return err
			}
			return respondWithMessage("Message sent")

		}
		return fmt.Errorf("unhandled command option: %s %s", name, cmdOption.Name)

	}
	return fmt.Errorf("unhandled application command %s", name)
}

func (b *Bot) handleMessageComponent(i *discordgo.InteractionCreate) error {
	respondWithUpdate := func(content string) error {
		err := b.ds.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseUpdateMessage,
			Data: &discordgo.InteractionResponseData{
				Content: content,
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return err
	}
	if i.Type != discordgo.InteractionMessageComponent {
		return nil
	}
	userID, err := interactionUserID(i)
	if err != nil {
		return err
	}
	data := i.MessageComponentData()
	customID := data.CustomID
	if customID == idNewReminder {
		seconds, err := strconv.Atoi(data.Values[0])
		if err != nil {
			return err
		}
		var dueAt time.Time
		if seconds > 0 {
			dueAt = time.Now().UTC().Add(time.Second * time.Duration(seconds))
		}
		mr := i.Message.MessageReference
		uid := messageUID(mr.GuildID, mr.ChannelID, mr.MessageID)
		x, _ := b.messageCache.Load(uid)
		b.messageCache.Delete(uid)
		m := x.(discordMessage)
		id, created, err := b.st.UpdateOrCreateBookmark(storage.UpdateOrCreateBookmarkParams{
			AuthorID:  m.authorID,
			ChannelID: m.channelID,
			Content:   m.content,
			DueAt:     dueAt,
			GuildID:   m.guildID,
			MessageID: m.messageID,
			Timestamp: m.timestamp,
			UserID:    userID,
		})
		if err != nil {
			return err
		}
		var s1, s2 string
		if created {
			s1 = "created"
		} else {
			s1 = "updated"
		}
		if seconds > 0 {
			s2 = fmt.Sprintf("Will remind you in %s.", units.HumanDuration(time.Until(dueAt)))
		} else {
			s2 = "Will not remind you."
		}
		return respondWithUpdate(fmt.Sprintf("Bookmark #%d %s. %s", id, s1, s2))

	} else if customID == idCancelRemove {
		return respondWithUpdate("Canceled")

	} else if x, found := strings.CutPrefix(customID, idRemoveBookmark); found {
		id, err := strconv.Atoi(x)
		if err != nil {
			return err
		}
		if err := b.st.DeleteBookmark(int64(id)); err != nil {
			return err
		}
		return respondWithUpdate(fmt.Sprintf("Bookmark #%d removed", id))
	} else if x, found := strings.CutPrefix(customID, idSetReminder); found {
		id, err := strconv.Atoi(x)
		if err != nil {
			return err
		}
		seconds, err := strconv.Atoi(data.Values[0])
		if err != nil {
			return err
		}
		var dueAt time.Time
		if seconds > 0 {
			dueAt = time.Now().UTC().Add(time.Second * time.Duration(seconds))
		}
		if err := b.st.SetReminder(int64(id), dueAt); err != nil {
			return err
		}
		var s string
		if seconds > 0 {
			s = "set"
		} else {
			s = "removed"
		}
		return respondWithUpdate(fmt.Sprintf("Reminder %s for bookmark #%d", s, id))
	}
	return fmt.Errorf("unhandled custom ID %s", customID)
}

// func (b *Bot) removeCommands() error {
// 	for id, name := range b.cmdIDs {
// 		err := b.s.ApplicationCommandDelete(appID, "", id)
// 		if err != nil {
// 			return fmt.Errorf("failed to delete command %s: %w", name, err)
// 		}
// 	}
// 	return nil
// }

// func (b *Bot) sendMessage() error {
// 	channelID := "1107650067139141672"
// 	messageID := "1267573916818346129"

// 	url := fmt.Sprintf("https://discord.com/channels/%s/%s/%s", "", channelID, messageID)
// 	_, err := b.s.ChannelMessageSend(channelID, "Hello\n"+url)
// 	if err != nil {
// 		return err
// 	}
// 	return nil
// }

type makeEmbedFromBookmarkOpts struct {
	hideDue bool
}

func (b *Bot) makeEmbedFromBookmark(bm queries.Bookmark, opts makeEmbedFromBookmarkOpts) *discordgo.MessageEmbed {
	var guildID string
	if bm.GuildID == "" {
		guildID = "@me"
	} else {
		guildID = bm.GuildID
	}
	user, err := b.fetchUser(bm.AuthorID)
	if err != nil {
		panic(err)
	}
	messageLink := fmt.Sprintf("https://discord.com/channels/%s/%s/%s", guildID, bm.ChannelID, bm.MessageID)
	me := &discordgo.MessageEmbed{
		Author: &discordgo.MessageEmbedAuthor{
			Name:    user.Name,
			IconURL: user.AvatarURL,
		},
		Footer: &discordgo.MessageEmbedFooter{
			Text: fmt.Sprintf("#%d", bm.ID),
		},
		Description: fmt.Sprintf("%s\n\n%s", bm.Content, messageLink),
		Timestamp:   bm.Timestamp.Format(time.RFC3339),
	}
	if !opts.hideDue && bm.DueAt.Valid {
		me.Description += fmt.Sprintf("\n\n🕘 **Due in %s**", units.HumanDuration(time.Until(bm.DueAt.Time)))
		me.Color = colorOrange
	}
	return me
}

func (b *Bot) fetchUser(userID string) (User, error) {
	var user User
	x, ok := b.userCache.Load(userID)
	if !ok {
		u, err := b.ds.User(userID)
		if err != nil {
			return User{}, err
		}
		user = User{Name: u.DisplayName(), ID: userID, AvatarURL: u.AvatarURL("")}
		b.userCache.Store(userID, user)
	} else {
		user = x.(User)
	}
	return user, nil
}
