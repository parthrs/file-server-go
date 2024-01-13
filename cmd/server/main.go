package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"file-server-go/pkg/fileserver"

	"github.com/rs/zerolog/log"
)

// main is the entrypoint for the fileserver
// program to start and serve requests
func main() {
	fs, err := fileserver.NewFileService()
	if err != nil {
		log.Err(err).Msg("Error creating service, exiting..")
		return
	}
	err = fs.Start()
	if err != nil {
		log.Err(err).Msg("Error starting service, exiting..")
		return
	}

	// Catch user interrupts (and K8s pod kill requests - SIGTERM)
	// https://stackoverflow.com/a/72085533
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, syscall.SIGTERM)

	for {
		select {
		case <-interrupt:
			fs.Stop(context.Background())
			return
		default:
			time.Sleep(time.Second * 1)
			continue
		}
	}
}
