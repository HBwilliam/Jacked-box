import asyncio
import websockets
import json

async def receiver(websocket):
    try:
        async for message in websocket:
            print("\n[Received]:", message)
    except websockets.ConnectionClosed:
        print("Disconnected from server.")

async def sender(websocket):
    while True:
        message = input("Message: ")
        if message == "/quit":
            await websocket.close()
            break
        elif message:
            await websocket.send(json.dumps({"type": "message", "content": message}))

async def interact():
    uri = "ws://localhost:8765"
    async with websockets.connect(uri) as websocket:
        print("Connected to server.")
        
        while True:
            print("\nOptions:")
            print("1. Create Lobby")
            print("2. Join Lobby")
            print("3. Quit")

            choice = input("Select: ")

            if choice == "1":
                print("\nGame Modes:")
                print("1. Blackjack")
                print("2. Quit")

                choice = input("Select: ")

                if choice == "1":
                    game_mode = "blackjack"
                    await websocket.send(json.dumps({"type": "create", "game_mode": game_mode}))
                    response = await websocket.recv()
                    print("Response:", response)
                elif choice == "2":
                    print("Quitting...")
                    continue
                else:
                    print("Invalid option.")
                    continue

            elif choice == "2":
                lobby_id = input("Lobby ID: ")
                await websocket.send(json.dumps({"type": "join", "lobby_id": lobby_id}))
                response = await websocket.recv()
                print("Response:", response)
                print("You can now send messages. Type /quit to leave.")
                await asyncio.gather(sender(websocket), receiver(websocket))
                break  # End main menu after chat session

            elif choice == "3":
                print("Disconnecting...")
                break

            else:
                print("Invalid option.")

asyncio.run(interact())
