// Package main assembles and runs the calendar notification storer service.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	internalconfig "github.com/isemibratov/golang-hw/hw12_13_14_15_16_calendar/internal/config"
	kafkaclient "github.com/isemibratov/golang-hw/hw12_13_14_15_16_calendar/internal/kafka"
	"github.com/isemibratov/golang-hw/hw12_13_14_15_16_calendar/internal/logger"
	sqlstorage "github.com/isemibratov/golang-hw/hw12_13_14_15_16_calendar/internal/storage/sql"
	"github.com/isemibratov/golang-hw/hw12_13_14_15_16_calendar/internal/storer"
)

const (
	defaultConfigPath     = "/etc/calendar/storer.toml"
	storageConnectTimeout = 10 * time.Second
	shutdownTimeout       = 3 * time.Second
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "calendar_storer: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	flags := flag.NewFlagSet("calendar_storer", flag.ContinueOnError)
	configFile := flags.String("config", defaultConfigPath, "path to configuration file")
	if err := flags.Parse(args); err != nil {
		return fmt.Errorf("parse command line: %w", err)
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected arguments: %v", flags.Args())
	}

	config, err := internalconfig.LoadStorer(*configFile)
	if err != nil {
		return err
	}
	logg, err := logger.New(config.Logger.Level)
	if err != nil {
		return fmt.Errorf("create logger: %w", err)
	}

	ctx, cancel := signal.NotifyContext(
		context.Background(),
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGHUP,
	)
	defer cancel()

	notificationStorage, err := connectStorage(ctx, config.Storage.DSN)
	if err != nil {
		return err
	}
	defer closeStorage(logg, notificationStorage)

	if err = kafkaclient.EnsureTopic(
		ctx,
		config.Kafka.Brokers,
		config.Kafka.Topic,
		config.Kafka.ConnectTimeout.Value(),
		config.Kafka.RetryInitial.Value(),
		config.Kafka.RetryMax.Value(),
		logg,
	); err != nil {
		if errors.Is(err, context.Canceled) {
			return nil
		}
		return fmt.Errorf("prepare Kafka topic: %w", err)
	}

	consumer := kafkaclient.NewConsumer(
		config.Kafka.Brokers,
		config.Kafka.Topic,
		config.Kafka.GroupID,
		config.Kafka.MaxMessageBytes,
	)
	defer func() {
		if closeErr := consumer.Close(); closeErr != nil {
			logg.Error("failed to close Kafka consumer: " + closeErr.Error())
		}
	}()

	process, err := storer.New(consumer, notificationStorage, logg)
	if err != nil {
		return fmt.Errorf("create storer: %w", err)
	}
	logg.Info("calendar storer is starting")
	if err = process.Run(ctx); err != nil {
		return fmt.Errorf("run storer: %w", err)
	}
	return nil
}

func connectStorage(ctx context.Context, dsn string) (*sqlstorage.Storage, error) {
	connectCtx, cancel := context.WithTimeout(ctx, storageConnectTimeout)
	defer cancel()

	client := sqlstorage.New(dsn)
	if err := client.Connect(connectCtx); err != nil {
		return nil, fmt.Errorf("connect SQL storage: %w", err)
	}
	return client, nil
}

func closeStorage(logg *logger.Logger, client *sqlstorage.Storage) {
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := client.Close(ctx); err != nil {
		logg.Error("failed to close storage: " + err.Error())
	}
}
