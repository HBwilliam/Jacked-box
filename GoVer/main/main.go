package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	gover "jackedBox"
)

func main() {
	// Initialize the game manager
	gm, err := gover.NewGameManager("0.0.0.0:4222")
	if err != nil {
		log.Fatal(err)
	}

	// Start the game manager
	gm.Start()

	// Set up graceful shutdown on system signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	log.Println("Game server running... Press Ctrl+C to shutdown")

	// Block until we receive a signal
	<-sigChan

	log.Println("Received shutdown signal...")
	gm.Shutdown()
}
