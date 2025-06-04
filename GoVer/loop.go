package gover

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/nats-io/nats.go"
)

// NewGameManager creates a new game manager with NATS connection
func NewGameManager(natsURL string) (*GameManager, error) {
	nc, err := nats.Connect(natsURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to NATS: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	gm := &GameManager{
		lobbies:  make(map[string]*Lobby),
		natsConn: nc,
		ctx:      ctx,
		cancel:   cancel,
	}

	// Start listening for NATS messages
	if err := gm.setupNATSSubscriptions(); err != nil {
		nc.Close()
		return nil, fmt.Errorf("failed to setup NATS subscriptions: %w", err)
	}

	return gm, nil
}

// setupNATSSubscriptions sets up all NATS message subscriptions
func (gm *GameManager) setupNATSSubscriptions() error {
	// Subscribe to player actions
	_, err := gm.natsConn.Subscribe("game.action.*", func(msg *nats.Msg) {
		gm.handleNATSMessage(msg)
	})
	if err != nil {
		return err
	}

	// Subscribe to lobby management
	_, err = gm.natsConn.Subscribe("lobby.*", func(msg *nats.Msg) {
		gm.handleNATSMessage(msg)
	})
	if err != nil {
		return err
	}

	return nil
}

// handleNATSMessage processes incoming NATS messages
func (gm *GameManager) handleNATSMessage(msg *nats.Msg) {
	var natsMsg NATSMessage
	if err := json.Unmarshal(msg.Data, &natsMsg); err != nil {
		log.Printf("Error unmarshaling NATS message: %v", err)
		return
	}

	switch natsMsg.Type {
	case PlayerJoinMsg:
		gm.handlePlayerJoin(natsMsg)
	case PlayerLeaveMsg:
		gm.handlePlayerLeave(natsMsg)
	case PlayerActionMsg:
		gm.handlePlayerAction(natsMsg)
	case LobbyCreateMsg:
		gm.handleLobbyCreate(natsMsg)
	case LobbyCloseMsg:
		gm.handleLobbyClose(natsMsg)
	case StartGameMsg:
		gm.handleStartGame(natsMsg)
	}
}

// CreateLobby creates a new game lobby
func (gm *GameManager) CreateLobby(lobbyID string) (*Lobby, error) {
	gm.mutex.Lock()
	defer gm.mutex.Unlock()

	if _, exists := gm.lobbies[lobbyID]; exists {
		return nil, fmt.Errorf("lobby %s already exists", lobbyID)
	}

	game := &Game{
		ID:      lobbyID,
		Players: make(map[string]*Player),
		Dealer:  Player{ID: "dealer", Name: "Dealer"},
		Deck:    *InitDeck(),
		State:   Waiting,
	}

	lobby := &Lobby{
		ID:             lobbyID,
		Game:           game,
		ActionChan:     make(chan PlayerAction, 100),
		Quit:           make(chan struct{}),
		PlayerStates:   make(map[string]PlayerState),
		RestartTimer:   nil,
		WaitingPlayers: make(map[string]*Player),
	}

	gm.lobbies[lobbyID] = lobby

	// Start the lobby's game loop
	gm.wg.Add(1)
	go gm.runLobbyGameLoop(lobby)

	log.Printf("Created lobby: %s", lobbyID)
	return lobby, nil
}

// runLobbyGameLoop runs the main game loop for a specific lobby
func (gm *GameManager) runLobbyGameLoop(lobby *Lobby) {
	defer gm.wg.Done()
	defer func() {
		gm.mutex.Lock()
		delete(gm.lobbies, lobby.ID)
		gm.mutex.Unlock()
		lobby.Close()
		log.Printf("Closed lobby: %s", lobby.ID)
	}()

	ticker := time.NewTicker(100 * time.Millisecond) // Game tick rate
	defer ticker.Stop()

	for {
		select {
		case <-gm.ctx.Done():
			return
		case <-lobby.Quit:
			return
		case action := <-lobby.ActionChan:
			gm.processGameAction(lobby, action)
		case <-ticker.C:
			gm.updateGameState(lobby)
		}
	}
}

// processGameAction handles player actions within a lobby
func (gm *GameManager) processGameAction(lobby *Lobby, action PlayerAction) {
	game := lobby.Game

	// Only process actions if game is in progress
	if game.State != InProgress {
		return
	}

	player, exists := game.Players[action.PlayerID]
	if !exists {
		return
	}

	// Check if player is still in playing state
	if lobby.PlayerStates[action.PlayerID] != PlayerPlaying {
		log.Printf("Ignoring action: player %s is in state %s", action.PlayerID, lobby.PlayerStates[action.PlayerID])
		return
	}

	switch action.Action {
	case HitAct:
		Hit(player, &game.Deck)
		log.Printf("Player %s hits and receives card: %v", player.ID, player.Hand[len(player.Hand)-1])
		score := CalculateScore(player)

		// Check if player busted
		if score > 21 {
			lobby.PlayerStates[action.PlayerID] = PlayerBusted
			log.Printf("Player %s busted with score %d", player.ID, score)
		}

	case StandAct:
		gm.stand(lobby, action.PlayerID)
		log.Printf("Player %s stands with score %d", player.ID, CalculateScore(player))
	}

	// Check if all players have finished their turns
	if gm.allPlayersFinished(lobby) {
		gm.processDealerTurn(game)
		gm.determineWinners(game)
		game.State = Finished

		// Set up restart timer (only once)
		if lobby.RestartTimer == nil {
			lobby.RestartTimer = time.AfterFunc(10*time.Second, func() {
				gm.restartGame(lobby)
			})
		}
	}

	// Broadcast game update
	gm.broadcastGameUpdate(lobby)
}

// stand implements the stand logic for a player
func (gm *GameManager) stand(lobby *Lobby, playerID string) {
	// Set player state to stood
	lobby.PlayerStates[playerID] = PlayerStood
	log.Printf("Player %s has stood", playerID)
}

// updateGameState performs periodic game state updates
func (gm *GameManager) updateGameState(lobby *Lobby) {
	game := lobby.Game

	switch game.State {
	case Waiting:
		// Don't auto-start games anymore - wait for manual start
		// Players can join and the first player can start the game
	case InProgress:
		// Check for timeouts, disconnected players, etc.
		// This is where you'd implement turn timers
	case Finished:
		// Game is finished, restart timer should be handling restart
		// No need for additional logic here to prevent infinite loops
	}
}

// startGame initializes a new game round
func (gm *GameManager) startGame(lobby *Lobby) {
	game := lobby.Game
	log.Printf("Starting game in lobby: %s", game.ID)

	// Cancel any existing restart timer
	if lobby.RestartTimer != nil {
		lobby.RestartTimer.Stop()
		lobby.RestartTimer = nil
	}

	// Reset deck if needed
	if len(game.Deck.Cards) < 10 { // Threshold for reshuffling
		game.Deck = *InitDeck()
	}

	// Clear all hands
	for _, player := range game.Players {
		player.Hand = []Card{}
	}
	game.Dealer.Hand = []Card{}

	// Initialize all players to playing state
	for playerID := range game.Players {
		lobby.PlayerStates[playerID] = PlayerPlaying
	}

	// Deal initial cards
	for _, player := range game.Players {
		InitDeal(player, &game.Deck)
	}
	InitDeal(&game.Dealer, &game.Deck)

	game.State = InProgress

	// Broadcast game start
	gm.broadcastGameUpdate(lobby)
}

// restartGame resets the game for a new round
func (gm *GameManager) restartGame(lobby *Lobby) {
	game := lobby.Game
	log.Printf("Restarting game in lobby: %s", game.ID)

	// Reset restart timer
	lobby.RestartTimer = nil

	// Move waiting players to active players
	for playerID, player := range lobby.WaitingPlayers {
		game.Players[playerID] = player
		delete(lobby.WaitingPlayers, playerID)
	}

	// Only restart if there are still players in the lobby
	if len(game.Players) > 0 {
		game.State = Waiting
		// Broadcast the updated state
		gm.broadcastGameUpdate(lobby)
	} else {
		// No players left, close the lobby
		close(lobby.Quit)
	}
}

// allPlayersFinished checks if all players have finished their turns
func (gm *GameManager) allPlayersFinished(lobby *Lobby) bool {
	// Check if all players are either stood or busted
	for playerID := range lobby.Game.Players {
		state, exists := lobby.PlayerStates[playerID]
		if !exists || state == PlayerPlaying {
			return false
		}
	}
	return true
}

// processDealerTurn handles the dealer's turn
func (gm *GameManager) processDealerTurn(game *Game) {
	log.Printf("Processing dealer turn in lobby: %s", game.ID)
	DealerTurn(&game.Dealer, &game.Deck)
}

// determineWinners calculates and logs the winners
func (gm *GameManager) determineWinners(game *Game) {
	dealerScore := CalculateScore(&game.Dealer)
	dealerBusted := dealerScore > 21

	for playerID, player := range game.Players {
		playerScore := CalculateScore(player)
		playerBusted := playerScore > 21

		var result string
		if playerBusted {
			result = "lost (busted)"
		} else if dealerBusted {
			result = "won (dealer busted)"
		} else if playerScore > dealerScore {
			result = "won"
		} else if playerScore == dealerScore {
			result = "tied"
		} else {
			result = "lost"
		}

		log.Printf("Player %s %s (Score: %d, Dealer: %d)", playerID, result, playerScore, dealerScore)
	}
}

// broadcastGameUpdate sends game state to all clients via NATS
func (gm *GameManager) broadcastGameUpdate(lobby *Lobby) {
	// Determine if any player can start the game (only when waiting and has players)
	canStartGame := lobby.Game.State == Waiting && len(lobby.Game.Players) > 0

	// Create a message for waiting players
	var message string
	if lobby.Game.State == InProgress && len(lobby.WaitingPlayers) > 0 {
		message = "Game in progress, please wait until next round"
	}

	update := GameBroadcast{
		GameState:      lobby.Game.State,
		Players:        lobby.Game.Players,
		PlayerStates:   lobby.PlayerStates,
		Dealer:         &lobby.Game.Dealer,
		WaitingPlayers: lobby.WaitingPlayers,
		Message:        message,
		CanStartGame:   canStartGame,
	}

	msg := NATSMessage{
		Type:    GameUpdateMsg,
		LobbyID: lobby.ID,
		Data:    update,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("Error marshaling game update: %v", err)
		return
	}

	subject := fmt.Sprintf("game.update.%s", lobby.ID)
	if err := gm.natsConn.Publish(subject, data); err != nil {
		log.Printf("Error publishing game update: %v", err)
	}
}

// Handle NATS message handlers
func (gm *GameManager) handlePlayerJoin(msg NATSMessage) {
	playerData, ok := msg.Data.(map[string]interface{})
	if !ok {
		log.Printf("Invalid player data in join message")
		return
	}

	playerName, _ := playerData["name"].(string)
	player := &Player{
		ID:   msg.PlayerID,
		Name: playerName,
		Hand: []Card{},
	}

	gm.mutex.RLock()
	lobby, exists := gm.lobbies[msg.LobbyID]
	gm.mutex.RUnlock()

	if !exists {
		// Create lobby if it doesn't exist
		lobby, _ = gm.CreateLobby(msg.LobbyID)
	}

	// Check if game is in progress
	if lobby.Game.State == InProgress {
		// Add to waiting players
		lobby.WaitingPlayers[msg.PlayerID] = player
		log.Printf("Player %s joined lobby %s (waiting for next round)", msg.PlayerID, msg.LobbyID)
	} else {
		// Add to active players
		lobby.Game.AddPlayer(player)
		lobby.PlayerStates[msg.PlayerID] = PlayerPlaying
		log.Printf("Player %s joined lobby %s", msg.PlayerID, msg.LobbyID)
	}

	// Broadcast update to inform all players
	gm.broadcastGameUpdate(lobby)
}

func (gm *GameManager) handlePlayerLeave(msg NATSMessage) {
	gm.mutex.RLock()
	lobby, exists := gm.lobbies[msg.LobbyID]
	gm.mutex.RUnlock()

	if !exists {
		return
	}

	// Remove from active players
	lobby.Game.RemovePlayer(msg.PlayerID)
	delete(lobby.PlayerStates, msg.PlayerID)

	// Remove from waiting players if they were waiting
	delete(lobby.WaitingPlayers, msg.PlayerID)

	log.Printf("Player %s left lobby %s", msg.PlayerID, msg.LobbyID)

	// Close lobby if no players left (neither active nor waiting)
	if len(lobby.Game.Players) == 0 && len(lobby.WaitingPlayers) == 0 {
		// Cancel restart timer if it exists
		if lobby.RestartTimer != nil {
			lobby.RestartTimer.Stop()
		}
		close(lobby.Quit)
	} else {
		// Broadcast update to remaining players
		gm.broadcastGameUpdate(lobby)
	}
}

func (gm *GameManager) handlePlayerAction(msg NATSMessage) {
	actionData, ok := msg.Data.(map[string]interface{})
	if !ok {
		return
	}

	actionStr, _ := actionData["action"].(string)
	action := PlayerAction{
		PlayerID: msg.PlayerID,
		Action:   Action(actionStr),
	}

	gm.mutex.RLock()
	lobby, exists := gm.lobbies[msg.LobbyID]
	gm.mutex.RUnlock()

	if exists {
		select {
		case lobby.ActionChan <- action:
		default:
			log.Printf("Action channel full for lobby %s", msg.LobbyID)
		}
	}
}

func (gm *GameManager) handleLobbyClose(msg NATSMessage) {
	gm.mutex.RLock()
	lobby, exists := gm.lobbies[msg.LobbyID]
	gm.mutex.RUnlock()

	if exists {
		// Cancel restart timer if it exists
		if lobby.RestartTimer != nil {
			lobby.RestartTimer.Stop()
		}
		close(lobby.Quit)
	}
}

func (gm *GameManager) handleLobbyCreate(msg NATSMessage) {
	gm.CreateLobby(msg.LobbyID)
}

func (gm *GameManager) handleStartGame(msg NATSMessage) {
	gm.mutex.RLock()
	lobby, exists := gm.lobbies[msg.LobbyID]
	gm.mutex.RUnlock()

	if !exists {
		return
	}

	// Only allow game start if in waiting state and player is in the lobby
	if lobby.Game.State == Waiting {
		// Check if the player requesting start is actually in the game
		if _, isActivePlayer := lobby.Game.Players[msg.PlayerID]; isActivePlayer {
			if len(lobby.Game.Players) >= 1 {
				gm.startGame(lobby)
				log.Printf("Player %s started the game in lobby %s", msg.PlayerID, msg.LobbyID)
			}
		}
	}
}

// Start begins the game manager's main loop
func (gm *GameManager) Start() {
	log.Println("Game manager started")
	// The game manager is now running and will handle messages via NATS subscriptions
}

// Shutdown gracefully shuts down the game manager
func (gm *GameManager) Shutdown() {
	log.Println("Shutting down game manager...")

	gm.cancel() // Cancel context to stop all lobbies

	// Close all lobbies and cancel their restart timers
	gm.mutex.Lock()
	for _, lobby := range gm.lobbies {
		if lobby.RestartTimer != nil {
			lobby.RestartTimer.Stop()
		}
		close(lobby.Quit)
	}
	gm.mutex.Unlock()

	// Wait for all lobby goroutines to finish
	gm.wg.Wait()

	// Close NATS connection
	gm.natsConn.Close()

	log.Println("Game manager shut down complete")
}

// GetLobbyStatus returns the current status of a lobby
func (gm *GameManager) GetLobbyStatus(lobbyID string) (*GameBroadcast, error) {
	gm.mutex.RLock()
	lobby, exists := gm.lobbies[lobbyID]
	gm.mutex.RUnlock()

	if !exists {
		return nil, fmt.Errorf("lobby %s not found", lobbyID)
	}

	return &GameBroadcast{
		GameState:      lobby.Game.State,
		Players:        lobby.Game.Players,
		PlayerStates:   lobby.PlayerStates,
		Dealer:         &lobby.Game.Dealer,
		WaitingPlayers: lobby.WaitingPlayers,
		CanStartGame:   lobby.Game.State == Waiting && len(lobby.Game.Players) > 0,
	}, nil
}

// ListLobbies returns a list of all active lobbies
func (gm *GameManager) ListLobbies() []string {
	gm.mutex.RLock()
	defer gm.mutex.RUnlock()

	lobbies := make([]string, 0, len(gm.lobbies))
	for lobbyID := range gm.lobbies {
		lobbies = append(lobbies, lobbyID)
	}
	return lobbies
}
