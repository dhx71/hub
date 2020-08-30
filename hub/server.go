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
			log.Println("hub   | invalid token provided")
			return
		}
		c, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			w.WriteHeader(400)
			log.Println("hub   | upgrade failed:", err)
			return
		}
		defer c.Close()
		var req hubRequest
		err = c.ReadJSON(&req)
		if err != nil {
			log.Println("hub   | failed to parse request:", err)
			c.WriteJSON(hubResponse{"error", false, "failed to parse request"})
			return
		}
		if req.Type == "join" {
			err = handleJoin(req.Room, req.Password, c)
			if err != nil {
				log.Println("hub   | failed to enter room", err)
				return
			}
		} else {
			log.Println("hub   | unknown request type", req.Type)
			c.WriteJSON(hubResponse{"error", false, "unknown request type"})
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
		log.Println("hub   |", c.RemoteAddr().String(), "create room", roomName)
		room = &roomInfo{roomName, password, make([]*websocket.Conn, 0)}
		rooms[roomName] = room
	} else {
		log.Println("hub   |", c.RemoteAddr().String(), "trying to enter room", roomName, len(room.participants), "participants")
	}

	if password != room.password {
		lock.Unlock()
		log.Println("hub   |", c.RemoteAddr().String(), "invalid password provided")
		c.WriteJSON(hubResponse{"error", false, "invalid password"})
		return fmt.Errorf("invalid room password")
	}
	room.participants = append(room.participants, c)
	lock.Unlock()
	log.Println("hub   |", c.RemoteAddr().String(), "entered room", roomName, len(room.participants), "participants")
	c.WriteJSON(hubResponse{"joined", true, ""})

	for {
		mt, message, err := c.ReadMessage()
		if err != nil {
			log.Println("hub   |", c.RemoteAddr().String(), "read error", err, "Removing participant from room")
			lock.Lock()
			removeParticipant(room.participants, c)
			if len(room.participants) == 0 {
				log.Println("No more participant in room. Closing room", roomName)
				delete(rooms, roomName)
			}
			lock.Unlock()
			break
		}
		log.Printf("hub   | recv %d bytes", len(message))
		err = room.broadcast(c, mt, message)
	}
	return nil
}

func (room *roomInfo) broadcast(source *websocket.Conn, mt int, msg []byte) error {
	lock.Lock()
	defer lock.Unlock()
	toDelete := make([]*websocket.Conn, 0)

	for _, participant := range room.participants {
		if participant != source {
			log.Printf("hub   | sending %d bytes to participant %s in room %s\n", len(msg), participant.RemoteAddr().String(), room.name)
			err := participant.WriteMessage(mt, msg)
			if err != nil {
				toDelete = append(toDelete, participant)
			}
		}
	}
	for _, disconnected := range toDelete {
		room.participants = removeParticipant(room.participants, disconnected)
		if len(room.participants) == 0 {
			log.Println("No more participant in room. Closing room", room.name)
			delete(rooms, room.name)
		}
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
