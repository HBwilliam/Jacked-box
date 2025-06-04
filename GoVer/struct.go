package gover

import (
	"context"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
)

type Suit string
type Rank string
type GameState string
type Action string
type PlayerState string
type MessageType string

const (
	Spades   Suit = "Spades"
	Hearts   Suit = "Hearts"
	Diamonds Suit = "Diamonds"
	Clubs    Suit = "Clubs"
)

const (
	Ace   Rank = "Ace"
	Two   Rank = "2"
	Three Rank = "3"
	Four  Rank = "4"
	Five  Rank = "5"
	Six   Rank = "6"
	Seven Rank = "7"
	Eight Rank = "8"
	Nine  Rank = "9"
	Ten   Rank = "10"
	Jack  Rank = "Jack"
	Queen Rank = "Queen"
	King  Rank = "King"
)

const (
	Waiting    GameState = "Waiting"
	InProgress GameState = "InProgress"
	Finished   GameState = "Finished"
)

const (
	HitAct   Action = "hit"
	StandAct Action = "stand"
)

const (
	PlayerPlaying PlayerState = "playing"
	PlayerStood   PlayerState = "stood"
	PlayerBusted  PlayerState = "busted"
)

const (
	PlayerJoinMsg   MessageType = "player_join"
	PlayerLeaveMsg  MessageType = "player_leave"
	PlayerActionMsg MessageType = "player_action"
	GameUpdateMsg   MessageType = "game_update"
	LobbyCreateMsg  MessageType = "lobby_create"
	LobbyCloseMsg   MessageType = "lobby_close"
	StartGameMsg    MessageType = "start_game"
)

type Game struct {
	ID      string
	Players map[string]*Player
	Dealer  Player
	Deck    Deck
	Discard []Card
	State   GameState
}

type Lobby struct {
	ID             string
	Game           *Game
	ActionChan     chan PlayerAction
	Quit           chan struct{}
	PlayerStates   map[string]PlayerState
	RestartTimer   *time.Timer
	WaitingPlayers map[string]*Player
}

type PlayerAction struct {
	PlayerID string
	Action   Action
}

type Card struct {
	Suit Suit
	Rank Rank
}

type Deck struct {
	Cards []Card
}

type Player struct {
	ID   string
	Name string
	Hand []Card
}

type GameManager struct {
	lobbies  map[string]*Lobby
	natsConn *nats.Conn
	mutex    sync.RWMutex
	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup
}

type NATSMessage struct {
	Type     MessageType `json:"type"`
	LobbyID  string      `json:"lobby_id"`
	PlayerID string      `json:"player_id,omitempty"`
	Data     interface{} `json:"data,omitempty"`
}

type GameBroadcast struct {
	GameState      GameState              `json:"game_state"`
	Players        map[string]*Player     `json:"players"`
	PlayerStates   map[string]PlayerState `json:"player_states"`
	Dealer         *Player                `json:"dealer"`
	Turn           string                 `json:"current_turn,omitempty"`
	WaitingPlayers map[string]*Player     `json:"waiting_players,omitempty"`
	Message        string                 `json:"message,omitempty"`
	CanStartGame   bool                   `json:"can_start_game,omitempty"`
}

func (l *Lobby) Close() {
	if l.RestartTimer != nil {
		l.RestartTimer.Stop()
	}
	// ActionChan and Quit channels will be closed by the caller
}

func (lobby *Lobby) Run() {
	for {
		select {
		case action := <-lobby.ActionChan:
			processPlayerAction(lobby.Game, action)
		case <-lobby.Quit:
			return // Exit the loop and close the lobby
		}
	}
}

// Process the player's action based on the action type
func processPlayerAction(game *Game, action PlayerAction) {
	player, exists := game.Players[action.PlayerID]
	if !exists {
		return // Player not found
	}
	switch action.Action {
	case HitAct:
		Hit(player, &game.Deck)
	case StandAct:
		Stand(player)
	default:
		return // Unknown action
	}
}

func (game *Game) AddPlayer(player *Player) {
	if game.Players == nil {
		game.Players = make(map[string]*Player)
	}
	game.Players[player.ID] = player
}

func (game *Game) RemovePlayer(playerID string) {
	if game.Players != nil {
		delete(game.Players, playerID)
	}
}

func (game *Game) DicsardHand(player *Player) {
	if player.Hand != nil && len(player.Hand) > 0 {
		game.Discard = append(game.Discard, player.Hand...)
		player.Hand = []Card{} // Clear the player's hand
	}
}
