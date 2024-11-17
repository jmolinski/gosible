package main

import (
	"context"
	"github.com/hhkbp2/go-logging"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/scylladb/gosible/command"
)

func main() {
	defer logging.Shutdown()
	err := execGosible()
	if err != nil {
		log.Fatal(err)
	}
}

func execGosible() error {
	app := &command.App{
		Context: contextProcess(),
	}

	rootCmd := NewCommand(app)

	if err := rootCmd.Execute(); err != nil {
		return err
	}
	return nil
}

func contextProcess() context.Context {
	ch := make(chan os.Signal, 1)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-ch
		cancel()
	}()
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	return ctx
}
