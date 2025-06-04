package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nats-io/nats.go"
)

type MessageType string
type NATSMessage struct {
	Type     MessageType `json:"type"`
	LobbyID  string      `json:"lobby_id"`
	PlayerID string      `json:"player_id,omitempty"`
	Data     interface{} `json:"data,omitempty"`
}

func main() {
	fmt.Println("🔍 Game Manager Monitor")
	fmt.Println("========================")
	fmt.Println("Monitoring all NATS traffic for the game manager...")
	fmt.Println("Press Ctrl+C to stop")
	fmt.Println()

	// Connect to NATS
	nc, err := nats.Connect("nats://localhost:4222")
	if err != nil {
		log.Fatal("Failed to connect to NATS:", err)
	}
	defer nc.Close()

	// Subscribe to all game-related topics
	subscriptions := []string{
		"game.>",  // All game messages
		"lobby.>", // All lobby messages
	}

	for _, subject := range subscriptions {
		nc.Subscribe(subject, func(msg *nats.Msg) {
			handleMessage(msg)
		})
	}

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Wait for shutdown signal
	<-sigChan
	fmt.Println("\n👋 Monitor shutting down...")
}

func handleMessage(msg *nats.Msg) {
	timestamp := time.Now().Format("15:04:05")

	fmt.Printf("[%s] 📨 Subject: %s\n", timestamp, msg.Subject)

	// Try to parse as JSON
	var natsMsg NATSMessage
	if err := json.Unmarshal(msg.Data, &natsMsg); err == nil {
		fmt.Printf("         Type: %s\n", natsMsg.Type)
		fmt.Printf("         LobbyID: %s\n", natsMsg.LobbyID)
		if natsMsg.PlayerID != "" {
			fmt.Printf("         PlayerID: %s\n", natsMsg.PlayerID)
		}
		if natsMsg.Data != nil {
			fmt.Printf("         Data: %+v\n", natsMsg.Data)
		}
	} else {
		// If it's not JSON, just print raw data
		fmt.Printf("         Raw: %s\n", string(msg.Data))
	}
	fmt.Println("         ---")
}
