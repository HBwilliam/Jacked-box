import asyncio
import websockets
import uuid
import json

class Player:
    def __init__(self, websocket, username):
        self.websocket = websocket
        self.username = username
        self.lobby = None

class Lobby:
    def __init__(self, lobby_id):
        self.id = lobby_id
        self.game_mode = None
        self.max_players = 8
        self.players = set()

    def add_player(self, player):
        self.players.add(player)
        player.lobby = self

    def set_game_mode(self, game_mode):
        self.game_mode = game_mode

    def remove_player(self, player):
        self.players.discard(player)
        player.lobby = None

lobbies = {}

async def handler(websocket):
    player = Player(websocket, username="guest_" + str(uuid.uuid4())[:8])

    async with websocket:
        print(f"Player {player.username} connected.")
        async for message in websocket:
            command = parse_message(message)

            if command["type"] == "create":
                lobby_id = str(uuid.uuid4())
                lobby = Lobby(lobby_id)
                game_mode = command["game_mode"]
                lobby.set_game_mode(game_mode)
                lobbies[lobby_id] = lobby
                await websocket.send(f"Lobby {lobby_id} created with game mode {game_mode}")
                print(f"Lobby {lobby_id} created with game mode {game_mode}")

            elif command["type"] == "join":
                lobby_id = command["lobby_id"]
                if lobby_id in lobbies:
                    lobbies[lobby_id].add_player(player)
                    await websocket.send(f"Joined lobby {lobby_id}")
                    print(f"Player {player.username} joined lobby {lobby_id}")
                else:
                    await websocket.send("Lobby not found")

            elif command["type"] == "message":
                if player.lobby:
                    await asyncio.gather(*[
                        p.websocket.send(f"{player.username}: {command['content']}")
                        for p in player.lobby.players if p != player
                    ])
                    print(f"Message from {player.username} in lobby {player.lobby.id}: {command['content']}")
                else:
                    await websocket.send("You are not in a lobby")
                    
def parse_message(message):
    return json.loads(message)

async def main():
    async with websockets.serve(handler, "localhost", 8765, ping_interval=None):
        await asyncio.Event().wait()  # Keep the server running

asyncio.run(main())
