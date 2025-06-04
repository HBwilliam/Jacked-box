package gover

import (
	"math/rand"
)

func NewStandardDeck() []Card {
	var deck []Card
	for _, suit := range []Suit{Spades, Hearts, Diamonds, Clubs} {
		for _, rank := range []Rank{Ace, Two, Three, Four, Five, Six, Seven, Eight, Nine, Ten, Jack, Queen, King} {
			deck = append(deck, Card{Suit: suit, Rank: rank})
		}
	}
	return deck
}

func ShuffleDeck(deck []Card) []Card {
	for i := len(deck) - 1; i > 0; i-- {
		j := rand.Intn(i + 1)
		deck[i], deck[j] = deck[j], deck[i]
	}
	return deck
}

func InitDeck() *Deck {
	deck := NewStandardDeck()
	shuffledDeck := ShuffleDeck(deck)
	return &Deck{Cards: shuffledDeck}
}

func InitDeal(Player *Player, deck *Deck) {
	// Deck size error handling
	if len(deck.Cards) < 2 {
		return // TO DO REMAKE DECK
	}
	Player.Hand = append(Player.Hand, deck.Cards[0], deck.Cards[1])
	// Remove the dealt cards from the deck
	deck.Cards = deck.Cards[2:]
}

func Hit(player *Player, deck *Deck) {
	// Deck size error handling
	if len(deck.Cards) == 0 {
		return // TO DO REMAKE DECK
	}
	player.Hand = append(player.Hand, deck.Cards[0])
	// Remove the dealt card from the deck
	deck.Cards = deck.Cards[1:]
}

func Stand(player *Player) {
	// Player stands, no action needed
	// This function can be expanded to handle any additional logic when a player stands
}

func DealerTurn(dealer *Player, deck *Deck) {
	// Dealer must hit until their score is 17 or higher
	for CalculateScore(dealer) < 17 {
		Hit(dealer, deck)
	}
	// If the dealer's score exceeds 21, they bust
	if CalculateScore(dealer) > 21 {
		// Handle bust logic (e.g., dealer loses)
	}
}

func CalculateScore(player *Player) int {
	score := 0
	aceCount := 0

	for _, card := range player.Hand {
		switch card.Rank {
		case Ace:
			score += 11
			aceCount++
		case Two:
			score += 2
		case Three:
			score += 3
		case Four:
			score += 4
		case Five:
			score += 5
		case Six:
			score += 6
		case Seven:
			score += 7
		case Eight:
			score += 8
		case Nine:
			score += 9
		case Ten, Jack, Queen, King:
			score += 10
		}
	}

	for score > 21 && aceCount > 0 {
		score -= 10 // Convert an Ace from 11 to 1
		aceCount--
	}

	return score
}
