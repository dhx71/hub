package hub

import (
	"fmt"
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

type hubRequest struct {
	Type, Room, Password string
}

type hubResponse struct {
	Type    string
	Success bool
	Cause   string
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
	log.Println("hubclt| entering room", room)
	hdrs := make(http.Header)
	hdrs["x-token"] = []string{client.token}
	roomInfo.conn, _, err = websocket.DefaultDialer.Dial(client.hubUrl, hdrs)
	if err != nil {
		return nil, err
	}
	log.Println("hubclt| connected to hub")
	err = roomInfo.WriteJSON(hubRequest{"join", room, password})
	if err != nil {
		roomInfo.Close()
		return nil, fmt.Errorf("failed to send join msg: %s", err)
	}
	var resp hubResponse
	err = roomInfo.ReadJSON(&resp)
	if err != nil {
		roomInfo.Close()
		return nil, fmt.Errorf("failed to read join confirmation msg: %s", err)
	}
	if !resp.Success {
		roomInfo.Close()
		return nil, fmt.Errorf("could not enter room. cause: %s", resp.Cause)
	}
	return roomInfo, nil
}

func (roomInfo *Room) Close() error {

	log.Println("hubclt| closing websocket for room", roomInfo.room)
	return roomInfo.conn.Close()
}

func (roomInfo *Room) WriteJSON(v interface{}) error {
	log.Printf("hubclt| Sending JSON %v to room: %s\n", v, roomInfo.room)
	return roomInfo.conn.WriteJSON(v)
}

func (roomInfo *Room) ReadJSON(v interface{}) error {
	err := roomInfo.conn.ReadJSON(v)
	if err != nil {
		log.Printf("hubclt| Error while reading from room %s. Err:%s", roomInfo.room, err)
		return err
	}
	log.Printf("hubclt| Read JSON %v from Room: %s\n", v, roomInfo.room)
	return nil
}

func (roomInfo *Room) Relay(netConn net.Conn) error {
	doClose := func() {
		log.Println("hubclt| closing room and net conn")
		roomInfo.Close()
		netConn.Close()
	}
	go func() {
		for {
			_, p, err := roomInfo.conn.ReadMessage()
			if err != nil {
				log.Printf("hubclt| failed to read data from room. Closing tunnel (room: %s). Err: %s\n", roomInfo.room, err)
				doClose()
				return
			}
			log.Printf("hubclt| read %d bytes from room\n", len(p))
			n, err := netConn.Write(p)
			if err != nil {
				log.Printf("hubclt| failed to write data to tcp connection. Closing tunnel (room: %s). Err: %s\n", roomInfo.room, err)
				doClose()
				return
			}
			log.Printf("hubclt| wrote %d bytes to tcp connection\n", n)
		}
	}()
	buf := make([]byte, 1500)
	for {
		n, err := netConn.Read(buf)
		if err != nil {
			log.Printf("hubclt| failed to read data from tcp connection %s. Closing tunnel (room: %s). Err: %s\n", netConn.RemoteAddr().String(), roomInfo.room, err)
			doClose()
			return err
		}
		log.Printf("hubclt| read %d bytes from tcp connection\n", n)
		err = roomInfo.conn.WriteMessage(websocket.BinaryMessage, buf[:n])
		if err != nil {
			log.Printf("hubclt| failed to write data to tunnel room %s. Closing tunnel. Err: %s\n", roomInfo.room, err)
			doClose()
			return err
		}
		log.Printf("hubctl| wrote %d bytes to room %s\n", n, roomInfo.room)
	}
}

func (roomInfo *Room) RemoteAddr() string {
	return roomInfo.conn.RemoteAddr().String()
}
