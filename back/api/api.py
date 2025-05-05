from fastapi import FastAPI, HTTPException
from pydantic import BaseModel
import sqlite3
from typing import List
import random
import string
from fastapi import Request
from fastapi.templating import Jinja2Templates
from fastapi.staticfiles import StaticFiles
from fastapi.middleware.cors import CORSMiddleware


app = FastAPI(title="API des Lobbys")
app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

app.mount("/static", StaticFiles(directory="static"), name="static")
templates = Jinja2Templates(directory="static")

DB = "db.sqlite3"

class Lobby(BaseModel):
    id: int
    code: str
    date_creation: str
    mode_de_jeu_id: int

class LobbyCreate(BaseModel):
    mode_de_jeu_id: int

class ModeDeJeu(BaseModel):
    id: int
    nom: str

#Load the lobby page
@app.get("/lobby/{code}")
def lobby_page(request: Request, code: str):
    with sqlite3.connect(DB) as conn:
        cursor = conn.cursor()
        cursor.execute("SELECT * FROM Lobby WHERE code = ?", (code,))
        row = cursor.fetchone()
        if row:
            lobby = Lobby(id=row[0], code=row[1], date_creation=row[2], mode_de_jeu_id=row[3])
            return templates.TemplateResponse("lobby.html", {
                "request": request,
                "code": code,
                "lobby": lobby
            })
        raise HTTPException(status_code=404, detail="Lobby non trouvé")
    
#Load the home page
@app.get("/")
def home_page(request: Request):
    with sqlite3.connect(DB) as conn:
        cursor = conn.cursor()
        cursor.execute("SELECT * FROM ModeDeJeu")
        rows = cursor.fetchall()
        modes_de_jeu = [ModeDeJeu(id=row[0], nom=row[1]) for row in rows]
    return templates.TemplateResponse("index.html", {
        "request": request,
        "modes_de_jeu": modes_de_jeu
    })

#Join a lobby
@app.put("/lobby")
def join_lobby(request: Request):
    form_data = request.form()
    username = form_data.get("username")
    code = form_data.get("code")

    if not username or not code:
        raise HTTPException(status_code=400, detail="Nom d'utilisateur ou code manquant")

    with sqlite3.connect(DB) as conn:
        cursor = conn.cursor()
        cursor.execute("SELECT * FROM Lobby WHERE code = ?", (code,))
        lobby = cursor.fetchone()
        if not lobby:
            raise HTTPException(status_code=404, detail="Lobby non trouvé")
    return {"message": f"Utilisateur '{username}' a rejoint le lobby '{code}' avec succès"}


# Create a new lobby
@app.post("/lobby", response_model=Lobby)
def create_lobby(lobby: LobbyCreate):
    print("creating lobby")
    with sqlite3.connect(DB) as conn:
        cursor = conn.cursor()
        def generate_unique_code():
            while True:
                code = ''.join(random.choices(string.ascii_uppercase, k=4))
                cursor.execute("SELECT 1 FROM Lobby WHERE code = ?", (code,))
                if not cursor.fetchone():
                    return code

        code = generate_unique_code()
        try:
            cursor.execute("INSERT INTO Lobby (code, mode_de_jeu_id) VALUES (?, ?)", 
                           (code, lobby.mode_de_jeu_id))
            conn.commit()
            lobby_id = cursor.lastrowid
            cursor.execute("SELECT * FROM Lobby WHERE id = ?", (lobby_id,))
            row = cursor.fetchone()
            return Lobby(id=row[0], code=row[1], date_creation=row[2], mode_de_jeu_id=row[3])
        except sqlite3.IntegrityError:
            raise HTTPException(status_code=400, detail="Code déjà utilisé ou mode de jeu invalide")
        
# delete a lobby
@app.delete("/lobby/{code}")
def delete_lobby(code: str):
    with sqlite3.connect(DB) as conn:
        cursor = conn.cursor()
        cursor.execute("SELECT * FROM Lobby WHERE code = ?", (code,))
        lobby = cursor.fetchone()
        if lobby is None:
            raise HTTPException(status_code=404, detail="Lobby non trouvé")
        cursor.execute("DELETE FROM Lobby WHERE code = ?", (code,))
        conn.commit()
        return {"message": f"Lobby '{code}' supprimé avec succès, ô Majesté."}
