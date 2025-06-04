package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/nats-io/nats.go"
)

type MessageType string

type Card struct {
	Suit string `json:"suit"`
	Rank string `json:"rank"`
}

type Player struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Hand []Card `json:"hand"`
}

type GameBroadcast struct {
	GameState      string             `json:"game_state"`
	Players        map[string]*Player `json:"players"`
	PlayerStates   map[string]string  `json:"player_states"`
	Dealer         *Player            `json:"dealer"`
	Turn           string             `json:"current_turn,omitempty"`
	WaitingPlayers map[string]*Player `json:"waiting_players,omitempty"`
	Message        string             `json:"message,omitempty"`
	CanStartGame   bool               `json:"can_start_game,omitempty"`
}

type NATSMessage struct {
	Type     MessageType `json:"type"`
	LobbyID  string      `json:"lobby_id"`
	PlayerID string      `json:"player_id,omitempty"`
	Data     interface{} `json:"data,omitempty"`
}

const (
	PlayerJoinMsg   MessageType = "player_join"
	PlayerLeaveMsg  MessageType = "player_leave"
	PlayerActionMsg MessageType = "player_action"
	GameUpdateMsg   MessageType = "game_update"
	StartGameMsg    MessageType = "start_game"
)

func main() {
	var playerName, lobbyID string
	fmt.Print("Enter your player name: ")
	fmt.Scanln(&playerName)
	fmt.Print("Enter lobby ID: ")
	fmt.Scanln(&lobbyID)

	playerID := fmt.Sprintf("%s-%d", playerName, time.Now().UnixNano())
	nc, err := nats.Connect(nats.DefaultURL)
	if err != nil {
		log.Fatalf("NATS connection failed: %v", err)
	}
	defer nc.Close()

	subject := fmt.Sprintf("game.update.%s", lobbyID)
	_, err = nc.Subscribe(subject, func(msg *nats.Msg) {
		var updateMsg NATSMessage
		if err := json.Unmarshal(msg.Data, &updateMsg); err != nil {
			log.Println("Failed to parse game update:", err)
			return
		}

		var broadcast GameBroadcast
		raw, _ := json.Marshal(updateMsg.Data)
		if err := json.Unmarshal(raw, &broadcast); err != nil {
			log.Println("Failed to parse GameBroadcast:", err)
			return
		}

		fmt.Println("\n===== Game Update =====")
		switch broadcast.GameState {
		case "InProgress":
			if player, ok := broadcast.Players[playerID]; ok {
				fmt.Printf("Your hand: %v\n", formatHand(player.Hand))
			} else {
				fmt.Println("Waiting for next round...")
			}
			if broadcast.Dealer != nil && len(broadcast.Dealer.Hand) > 0 {
				fmt.Printf("Dealer shows: %v\n", formatCard(broadcast.Dealer.Hand[0]))
			}
		case "Finished":
			fmt.Println("Game finished! Final results:")
			dealerScore := calculateScore(broadcast.Dealer.Hand)
			fmt.Printf("Dealer hand: %v (Score: %d)\n", formatHand(broadcast.Dealer.Hand), dealerScore)
			for id, player := range broadcast.Players {
				score := calculateScore(player.Hand)
				state := broadcast.PlayerStates[id]
				result := determineResult(state, score, dealerScore)
				fmt.Printf("Player %s: %v (Score: %d) -> %s\n", player.Name, formatHand(player.Hand), score, result)
			}
		default:
			fmt.Printf("Game State: %s\n", broadcast.GameState)
			fmt.Println(broadcast.Message)
		}
	})
	if err != nil {
		log.Fatalf("Failed to subscribe to updates: %v", err)
	}

	joinMsg := NATSMessage{
		Type:     PlayerJoinMsg,
		LobbyID:  lobbyID,
		PlayerID: playerID,
		Data: map[string]interface{}{
			"name": playerName,
		},
	}
	sendMsg(nc, "lobby.join", joinMsg)

	go func() {
		for {
			var input string
			fmt.Print("Enter command: ")
			fmt.Scanln(&input)
			input = strings.ToLower(strings.TrimSpace(input))

			switch input {
			case "hit", "stand":
				action := NATSMessage{
					Type:     PlayerActionMsg,
					LobbyID:  lobbyID,
					PlayerID: playerID,
					Data: map[string]interface{}{
						"action": input,
					},
				}
				sendMsg(nc, fmt.Sprintf("game.action.%s", lobbyID), action)

			case "start":
				startMsg := NATSMessage{
					Type:     StartGameMsg,
					LobbyID:  lobbyID,
					PlayerID: playerID,
				}
				sendMsg(nc, "lobby.start", startMsg)
				fmt.Println("Sent start game request...")

			default:
				fmt.Printf("Unknown command: %s\n", input)
				fmt.Println("Available: hit, stand, start, quit")
			}
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	<-sigCh

	leaveMsg := NATSMessage{
		Type:     PlayerLeaveMsg,
		LobbyID:  lobbyID,
		PlayerID: playerID,
	}
	sendMsg(nc, "lobby.leave", leaveMsg)
	fmt.Println("Exited.")
}

func sendMsg(nc *nats.Conn, subject string, msg NATSMessage) {
	data, err := json.Marshal(msg)
	log.Printf("Sending message on %s: %s", subject, data)
	if err != nil {
		log.Printf("Failed to marshal message: %v", err)
		return
	}
	if err := nc.Publish(subject, data); err != nil {
		log.Printf("Failed to send message on %s: %v", subject, err)
	}
}

func formatCard(card Card) string {
	return fmt.Sprintf("%s of %s", card.Rank, card.Suit)
}

func formatHand(hand []Card) string {
	formatted := ""
	for i, card := range hand {
		if i > 0 {
			formatted += ", "
		}
		formatted += formatCard(card)
	}
	return formatted
}

func calculateScore(hand []Card) int {
	score := 0
	aces := 0
	for _, card := range hand {
		switch card.Rank {
		case "Ace":
			score += 11
			aces++
		case "2", "3", "4", "5", "6", "7", "8", "9":
			score += int(card.Rank[0] - '0')
		case "10", "Jack", "Queen", "King":
			score += 10
		}
	}
	for score > 21 && aces > 0 {
		score -= 10
		aces--
	}
	return score
}

func determineResult(state string, playerScore, dealerScore int) string {
	if state == "busted" {
		return "Busted"
	}
	if dealerScore > 21 {
		return "Won (Dealer busted)"
	}
	if playerScore > dealerScore {
		return "Won"
	}
	if playerScore == dealerScore {
		return "Tied"
	}
	return "Lost"
}
