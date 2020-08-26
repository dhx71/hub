package hub

import (
	"fmt"
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

type roomInfo struct {
	name         string
	password     string
	participants []*websocket.Conn
}

var (
	upgrader = websocket.Upgrader{}
	rooms    = make(map[string]*roomInfo)
	lock     = sync.Mutex{}
)

func NewHubHandlerFunc(token string) func(w http.ResponseWriter, r *http.Request) {
	hubService := func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-token") != token {
			w.WriteHeader(401)
			log.Println("invalid token provided")
			return
		}
		c, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			w.WriteHeader(400)
			log.Println("upgrade failed:", err)
			return
		}
		defer c.Close()
		cmd := make(map[string]interface{})
		err = c.ReadJSON(&cmd)
		if err != nil {
			w.WriteHeader(400)
			log.Println("failed to parse command:", err)
			return
		}
		operation := cmd["type"]
		room := cmd["room"]
		roomPassword := cmd["password"]
		if operation == "join" {
			err = handleJoin(room.(string), roomPassword.(string), c)
			if err != nil {
				w.WriteHeader(401)
			}
		} else {
			w.WriteHeader(404)
			log.Println("unknown operation", operation)
			return
		}

	}
	return hubService
}

func handleJoin(roomName string, password string, c *websocket.Conn) error {
	lock.Lock()
	room, found := rooms[roomName]
	if !found {
		// first to join open the room
		log.Println("creating room", roomName)
		room = &roomInfo{roomName, password, make([]*websocket.Conn, 0)}
		rooms[roomName] = room
	} else {
		log.Println("trying to join room", roomName, len(room.participants), "participants")
	}

	if password != room.password {
		log.Println("invalid password provided")
		return fmt.Errorf("invalid room password")
	}
	room.participants = append(room.participants, c)
	lock.Unlock()
	log.Println("joined room", roomName, len(room.participants), "participants")

	for {
		mt, message, err := c.ReadMessage()
		if err == nil {
			log.Printf("recv %d bytes", len(message))
			err = room.broadcast(c, mt, message)
		}
		if err != nil {
			lock.Lock()
			defer lock.Unlock()
			log.Println("read error", err, "closing room...")
			for _, participant := range room.participants {
				participant.Close()
			}
			delete(rooms, roomName)
			break
		}
	}
	return nil
}

func (room *roomInfo) broadcast(source *websocket.Conn, mt int, msg []byte) error {
	lock.Lock()
	defer lock.Unlock()
	toDelete := make([]*websocket.Conn, 0)

	for _, participant := range room.participants {
		if participant != source {
			log.Printf("sending %d bytes\n", len(msg))
			err := participant.WriteMessage(mt, msg)
			if err != nil {
				toDelete = append(toDelete, participant)
			}
		}
	}
	for _, disconnected := range toDelete {
		room.participants = removeParticipant(room.participants, disconnected)
	}
	return nil
}

func removeParticipant(participants []*websocket.Conn, participantToRemove *websocket.Conn) []*websocket.Conn {
	for i, participant := range participants {
		if participantToRemove == participant {
			return removeIndex(participants, i)
		}
	}
	return participants
}

func removeIndex(slice []*websocket.Conn, s int) []*websocket.Conn {
	return append(slice[:s], slice[s+1:]...)
}
