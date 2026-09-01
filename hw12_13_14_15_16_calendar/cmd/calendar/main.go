package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/isemibratov/golang-hw/hw12_13_14_15_16_calendar/internal/app"
	"github.com/isemibratov/golang-hw/hw12_13_14_15_16_calendar/internal/logger"
	internalhttp "github.com/isemibratov/golang-hw/hw12_13_14_15_16_calendar/internal/server/http"
	memorystorage "github.com/isemibratov/golang-hw/hw12_13_14_15_16_calendar/internal/storage/memory"
	sqlstorage "github.com/isemibratov/golang-hw/hw12_13_14_15_16_calendar/internal/storage/sql"
)

const storageConnectTimeout = 10 * time.Second

func main() {
	if err := run(os.Args[1:]); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "calendar: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) > 0 && args[0] == "version" {
		printVersion()
		return nil
	}

	flags := flag.NewFlagSet("calendar", flag.ContinueOnError)
	configFile := flags.String("config", "/etc/calendar/config.toml", "path to configuration file")
	if err := flags.Parse(args); err != nil {
		return fmt.Errorf("parse command line: %w", err)
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected arguments: %v", flags.Args())
	}

	config, err := LoadConfig(*configFile)
	if err != nil {
		return err
	}

	logg, err := logger.New(config.Logger.Level)
	if err != nil {
		return fmt.Errorf("create logger: %w", err)
	}

	connectCtx, cancelConnect := context.WithTimeout(context.Background(), storageConnectTimeout)
	eventStorage, closeStorage, err := newStorage(connectCtx, config.Storage)
	cancelConnect()
	if err != nil {
		return err
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := closeStorage(ctx); err != nil {
			logg.Error("failed to close storage: " + err.Error())
		}
	}()

	calendar := app.New(logg, eventStorage)
	server := internalhttp.NewServer(logg, calendar, config.HTTP.Address())

	ctx, cancel := signal.NotifyContext(context.Background(),
		syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	defer cancel()

	logg.Info("calendar is starting at " + config.HTTP.Address())

	if err := server.Start(ctx); err != nil {
		return fmt.Errorf("run HTTP server: %w", err)
	}
	return nil
}

func newStorage(
	ctx context.Context,
	config StorageConf,
) (app.Storage, func(context.Context) error, error) {
	switch config.Type {
	case storageTypeMemory:
		return memorystorage.New(), func(context.Context) error { return nil }, nil
	case storageTypeSQL:
		storageClient := sqlstorage.New(config.DSN)
		if err := storageClient.Connect(ctx); err != nil {
			return nil, nil, fmt.Errorf("connect SQL storage: %w", err)
		}
		return storageClient, storageClient.Close, nil
	default:
		return nil, nil, fmt.Errorf("unsupported storage type %q", config.Type)
	}
}
