package main

import (
	"cmp"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"maps"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/joho/godotenv"

	"example/discord-remindme/internal/bot"
	"example/discord-remindme/internal/storage"
)

const (
	dbFileName = "bookmarker.sqlite"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}
	var (
		appIDFlag         = flag.String("app-id", os.Getenv("APP_ID"), "Discord app ID. Can be set by env.")
		botTokenFlag      = flag.String("bot-token", os.Getenv("BOT_TOKEN"), "Discord bot token. Can be set by env.")
		dataDirFlag       = flag.String("data-dir", "", "path to data files. Uses current directory if not set")
		resetDataFlag     = flag.Bool("reset-data", false, "resets all data")
		logLevelFlag      = flag.String("log-level", cmp.Or(os.Getenv("LOG_LEVEL"), "info"), "Set log level for this session. Can be set by env.")
		resetCommandsFlag = flag.Bool("reset-commands", false, "recreates Discord commands. Requires user re-install.")
	)
	flag.Parse()

	// set manual log level for this session if requested
	m := map[string]slog.Level{
		"debug": slog.LevelDebug,
		"info":  slog.LevelInfo,
		"warn":  slog.LevelWarn,
		"error": slog.LevelError,
	}
	l, ok := m[strings.ToLower(*logLevelFlag)]
	if !ok {
		fmt.Println("valid log levels are: ", strings.Join(slices.Collect(maps.Keys(m)), ", "))
		os.Exit(1)
	}
	log.SetFlags(log.LstdFlags | log.Llongfile)
	slog.SetLogLoggerLevel(l)
	slog.SetLogLoggerLevel(slog.LevelInfo)

	var dataDir string
	if *dataDirFlag != "" {
		dataDir = *dataDirFlag
	} else {
		p, err := os.Getwd()
		if err != nil {
			slog.Error("Failed to get current directory", "error", err)
			os.Exit(1)
		}
		dataDir = p
	}

	dbPath := filepath.Join(dataDir, dbFileName)
	if *resetDataFlag {
		err := deleteDatabaseFiles(dbPath)
		if err != nil {
			slog.Error("Failed to delete database files", "error", err)
			os.Exit(1)
		}
	}
	dsn := "file:///" + filepath.ToSlash(dbPath)
	dbRW, dbRO, err := storage.InitDB(dsn)
	if err != nil {
		slog.Error("Failed to initialize database", "dsn", dsn, "error", err)
		os.Exit(1)
	}
	defer dbRW.Close()
	defer dbRO.Close()
	st := storage.New(dbRW, dbRO)
	slog.Info("Connected to database")

	ds, err := discordgo.New("Bot " + *botTokenFlag)
	if err != nil {
		slog.Error("Failed to create Discord session", "error", err)
		os.Exit(1)
	}
	ds.Identify.Intents = discordgo.IntentDirectMessages
	b := bot.New(st, ds, *appIDFlag)
	if err := ds.Open(); err != nil {
		slog.Error("Cannot open the Discord session", "error", err)
		os.Exit(1)
	}
	defer ds.Close()

	if *resetCommandsFlag {
		err := b.ResetCommands()
		if err != nil {
			slog.Error("Failed to reset Discord commands", "error", err)
			os.Exit(1)
		}
	}

	b.Start()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	<-stop
	slog.Info("Graceful shutdown")
}

func deleteDatabaseFiles(dbPath string) error {
	files, err := filepath.Glob(dbPath + "*")
	if err != nil {
		return err
	}
	for _, f := range files {
		if err := os.Remove(f); err != nil {
			return err
		}
		slog.Info("file deleted", "name", f)
	}
	return nil
}
