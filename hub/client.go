package hub

import (
	"log"
	"net"
	"net/http"

	"github.com/gorilla/websocket"
	"github.com/mattn/go-ieproxy"
)

type Client struct {
	hubUrl, token string
}

type Room struct {
	room, password string
	conn           *websocket.Conn
}

func NewClient(hubUrl, token string, bypassproxy bool) *Client {
	if !bypassproxy {
		websocket.DefaultDialer = &websocket.Dialer{
			Proxy: ieproxy.GetProxyFunc()}
	}
	return &Client{hubUrl, token}
}

func (client *Client) Join(room, password string) (roomInfo *Room, err error) {
	roomInfo = &Room{room, password, nil}
	log.Println("joining room", room)
	hdrs := make(http.Header)
	hdrs["x-token"] = []string{client.token}
	roomInfo.conn, _, err = websocket.DefaultDialer.Dial(client.hubUrl, hdrs)
	if err != nil {
		return nil, err
	}
	log.Println("connected to hub")
	msg := make(map[string]interface{})
	msg["type"] = "join"
	msg["room"] = room
	msg["password"] = password
	err = roomInfo.conn.WriteJSON(msg)
	if err != nil {
		log.Fatal("failed to send join msg:", err)
	}
	return roomInfo, nil
}

func (roomInfo *Room) Close() error {
	return roomInfo.conn.Close()
}

func (roomInfo *Room) WriteJSON(v interface{}) error {
	return roomInfo.conn.WriteJSON(v)
}

func (roomInfo *Room) ReadJSON() (map[string]interface{}, error) {
	result := make(map[string]interface{})
	err := roomInfo.conn.ReadJSON(&result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (roomInfo *Room) Relay(netConn net.Conn) error {
	doClose := func() {
		roomInfo.conn.Close()
		netConn.Close()
	}
	go func() {
		for {
			_, p, err := roomInfo.conn.ReadMessage()
			if err != nil {
				log.Println("failed to read data from room. Closing tunnel")
				doClose()
				return
			}
			log.Printf("read %d bytes from room\n", len(p))
			n, err := netConn.Write(p)
			if err != nil {
				log.Println("failed to write data to tcp connection. Closing tunnel")
				doClose()
				return
			}
			log.Printf("wrote %d bytes to tcp connection\n", n)
		}
	}()
	for {
		buf := make([]byte, 1500)
		n, err := netConn.Read(buf)
		if err != nil {
			log.Println("failed to read data from tcp connection. Closing tunnel")
			doClose()
			return err
		}
		log.Printf("read %d bytes from tcp connection\n", n)
		err = roomInfo.conn.WriteMessage(websocket.BinaryMessage, buf)
		if err != nil {
			log.Println("failed to write data to tunnel room. Closing tunnel")
			doClose()
			return err
		}
		log.Printf("wrote %d bytes to room\n", n)
	}
}
