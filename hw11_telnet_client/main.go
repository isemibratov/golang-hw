package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"time"
)

const (
	defaultConnectionTimeout = 10 * time.Second
	// Let an in-flight server greeting reach stdout before stdin EOF closes the socket.
	shutdownGracePeriod = 200 * time.Millisecond
)

func main() {
	if err := run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr); err != nil {
		log.New(os.Stderr, "", 0).Print(err)
		os.Exit(1)
	}
}

func run(args []string, input io.ReadCloser, output, stderr io.Writer) error {
	flags := flag.NewFlagSet("go-telnet", flag.ContinueOnError)
	flags.SetOutput(stderr)
	timeout := flags.Duration("timeout", defaultConnectionTimeout, "connection timeout")

	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return fmt.Errorf("parse arguments: %w", err)
	}
	if flags.NArg() != 2 {
		return errors.New("usage: go-telnet [--timeout=duration] host port")
	}

	address := net.JoinHostPort(flags.Arg(0), flags.Arg(1))
	client := NewTelnetClient(address, *timeout, input, output)
	if err := client.Connect(); err != nil {
		return fmt.Errorf("connect to %s: %w", address, err)
	}

	logger := log.New(stderr, "", 0)
	logger.Printf("...Connected to %s", address)

	// Before this point SIGINT keeps its default behavior and interrupts a blocked Dial.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	return runSession(ctx, client, logger)
}

func runSession(ctx context.Context, client TelnetClient, logger *log.Logger) error {
	sendDone := make(chan error, 1)
	receiveDone := make(chan error, 1)

	go func() { receiveDone <- client.Receive() }()
	go func() { sendDone <- client.Send() }()

	var transferErr error
	var operation, message string
	select {
	case <-ctx.Done():
		message = "...Interrupted"
	case transferErr = <-sendDone:
		operation, message = "send", "...EOF"
		if transferErr == nil {
			select {
			case <-ctx.Done():
				message = "...Interrupted"
			case transferErr = <-receiveDone:
				if transferErr != nil {
					operation = "receive"
				}
			case <-time.After(shutdownGracePeriod):
			}
		}
	case transferErr = <-receiveDone:
		operation, message = "receive", "...Connection was closed by peer"
	}

	if err := client.Close(); err != nil {
		logger.Printf("...Close error: %v", err)
	}

	if transferErr != nil {
		return fmt.Errorf("%s: %w", operation, transferErr)
	}
	logger.Print(message)
	return nil
}
