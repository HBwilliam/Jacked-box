import asyncio
import websockets
import uuid
import json

class Player:
    def __init__(self, websocket, identifier):
        self.websocket = websocket
        self.identifier = identifier
        self.username = None
        self.lobby = None
        self.system = False
        self.admin = False

    def set_system(self, bool):
        self.system = True

    def set_admin(self, bool):
        self.admin = True
    
    def set_username(self, username):
        self.username = username

class Lobby:
    def __init__(self, lobby_id):
        self.id = lobby_id
        self.game_mode = None
        self.max_players = 8
        self.players = set()
        self.system = None

    def add_player(self, player):
        self.players.add(player)
        player.lobby = self

        if len(self.players) > self.max_players:
            raise Exception("Lobby is full")
        if len(self.players) == 1:
            player.set_admin(True)
            player.identifier = "admin_" + self.id

    def add_system_player(self, player):
        self.system = (player)
        player.lobby = self
        player.set_system(True)
        player.identifier = "system_" + self.id

    def set_game_mode(self, game_mode):
        self.game_mode = game_mode

    def remove_player(self, player):
        self.players.discard(player)
        player.lobby = None

lobbies = {}

async def handler(websocket):
    player = Player(websocket, identifier="player_" + str(uuid.uuid4())[:8])

    async with websocket:
        print(f"Player {player.identifier} connected.")
        async for message in websocket:
            command = parse_message(message)

            if command["type"] == "create":
                lobby_id = str(uuid.uuid4())[:4]
                lobby = Lobby(lobby_id)
                game_mode = command["game_mode"]
                lobby.set_game_mode(game_mode)
                lobbies[lobby_id] = lobby
                lobby.add_system_player(player)
                await broadcast_lobby_status(lobby)
                print(f"Player {player.identifier} created lobby {lobby_id} with game mode {game_mode}")
                print(f"Lobby {lobby_id} created with game mode {game_mode}")

            elif command["type"] == "join":
                lobby_id = command["lobby_id"]
                username = command.get("username")
                player.set_username(username)
                if lobby_id in lobbies:
                    lobbies[lobby_id].add_player(player)
                    await broadcast_lobby_status(lobbies[lobby_id])
                    await websocket.send(f"Joined lobby {lobby_id}")
                    print(f"Player {player.identifier} joined lobby {lobby_id}")
                else:
                    await websocket.send("Lobby not found")

            elif command["type"] == "message":
                if player.lobby:
                    await asyncio.gather(*[
                        p.websocket.send(f"{player.identifier}: {command['content']}")
                        for p in player.lobby.players if p != player
                    ])
                    print(f"Message from {player.identifier} in lobby {player.lobby.id}: {command['content']}")
                else:
                    await websocket.send("You are not in a lobby")

            elif command["type"] == "status":
                if player.lobby:
                    lobby_info = {
                        "lobby_id": player.lobby.id,
                        "players": [p.identifier for p in player.lobby.players],
                        "game_mode": player.lobby.game_mode,
                    }
                    await websocket.send(json.dumps({"type": "status", "data": lobby_info}))

                    
def parse_message(message):
    return json.loads(message)

async def broadcast_lobby_status(lobby):
    lobby_info = {
        "type": "status",
        "data": {
            "lobby_id": lobby.id,
            "players": [p.username for p in lobby.players],
            "game_mode": lobby.game_mode,
        }
    }
    await asyncio.gather(*[
        p.websocket.send(json.dumps(lobby_info))
        for p in lobby.players
    ])


async def main():
    async with websockets.serve(handler, "localhost", 8765, ping_interval=None):
        await asyncio.Event().wait()  # Keep the server running

asyncio.run(main())
