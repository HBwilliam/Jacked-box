import random

SUITS = ['♠', '♥', '♦', '♣']
RANKS = ['2', '3', '4', '5', '6', '7', '8', '9', '10', 'J', 'Q', 'K', 'A']

def calculate_hand_value(hand):
    value = 0
    aces = 0
    for card in hand:
        rank = card[:-1]  # 'A♠' -> 'A'
        if rank in ['J', 'Q', 'K']:
            value += 10
        elif rank == 'A':
            value += 11
            aces += 1
        else:
            value += int(rank)
    while value > 21 and aces:
        value -= 10
        aces -= 1
    return value

class BlackjackGame:
    def __init__(self):
        self.deck = self.create_deck()
        self.players = {}  # player_id: {'hand': [...], 'status': 'playing'|'stood'|'bust'}
        self.dealer_hand = []
        self.in_progress = False

    def create_deck(self):
        return [rank + suit for suit in SUITS for rank in RANKS] * 6  # 6 decks

    def shuffle_deck(self):
        random.shuffle(self.deck)

    def start_game(self, player_ids):
        self.in_progress = True
        self.deck = self.create_deck()
        self.shuffle_deck()
        self.players = {}
        for pid in player_ids:
            self.players[pid] = {'hand': [self.deck.pop(), self.deck.pop()], 'status': 'playing'}
        self.dealer_hand = [self.deck.pop(), self.deck.pop()]

    def player_hit(self, player_id):
        if player_id not in self.players:
            return
        if self.players[player_id]['status'] != 'playing':
            return
        self.players[player_id]['hand'].append(self.deck.pop())
        hand_value = calculate_hand_value(self.players[player_id]['hand'])
        if hand_value > 21:
            self.players[player_id]['status'] = 'bust'

    def player_stand(self, player_id):
        if player_id in self.players and self.players[player_id]['status'] == 'playing':
            self.players[player_id]['status'] = 'stood'

    def all_done(self):
        return all(status['status'] in ['stood', 'bust'] for status in self.players.values())

    def dealer_play(self):
        while calculate_hand_value(self.dealer_hand) < 17:
            self.dealer_hand.append(self.deck.pop())

    def get_results(self):
        dealer_value = calculate_hand_value(self.dealer_hand)
        results = {}
        for pid, pdata in self.players.items():
            player_value = calculate_hand_value(pdata['hand'])
            if pdata['status'] == 'bust':
                results[pid] = 'lose'
            elif dealer_value > 21:
                results[pid] = 'win'
            elif player_value > dealer_value:
                results[pid] = 'win'
            elif player_value < dealer_value:
                results[pid] = 'lose'
            else:
                results[pid] = 'push'  # tie
        return results

    def game_state(self):
        return {
            'players': {
                pid: {
                    'hand': pdata['hand'],
                    'status': pdata['status'],
                    'value': calculate_hand_value(pdata['hand'])
                }
                for pid, pdata in self.players.items()
            },
            'dealer': {
                'hand': self.dealer_hand,
                'value': calculate_hand_value(self.dealer_hand)
            },
            'in_progress': self.in_progress
        }
