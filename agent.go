package main

import (
	"log"
	"net"
	"os"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/hub/hub"
)

func JoinRoom(room, password string) (c *websocket.Conn, err error) { /*
		log.Println("joining room", room)
		hdrs := make(http.Header)
		hdrs["x-token"] = []string{*token}
		hubws := *agent
		if len(hubws) == 0 {
			hubws = *client
		}
		c, _, err = websocket.DefaultDialer.Dial(hubws, hdrs)
		if err != nil {
			log.Fatal("dial:", err)
		}
		log.Println("connected to hub")
		msg := make(map[string]interface{})
		msg["type"] = "join"
		msg["room"] = room
		msg["password"] = password
		err = c.WriteJSON(msg)
		if err != nil {
			log.Fatal("failed to send join msg:", err)
		}
		return c, nil*/
	return nil, nil
}

func startAgent() {
	hubClient := hub.NewClient(*agent, *token, *bypassProxy)
	controlRoom, err := hubClient.Join(*room, *password)
	if err != nil {
		log.Fatal("failed to join room. ", *room)
	}
	defer controlRoom.Close()
	for {
		msg, err := controlRoom.ReadJSON()
		if err != nil {
			log.Println("failed to read JSON. ", err)
			continue
		}
		log.Printf("got message %v\n", msg)
		if msg["type"] == "createTunnel" {
			go createTunnel(hubClient, controlRoom, msg["destination"].(string), msg["refid"].(string))
		}
	}
	/*
		log.Println("starting agent and connecting to", *agent)
		if !*bypassProxy {
			websocket.DefaultDialer = &websocket.Dialer{
				Proxy: ieproxy.GetProxyFunc()}
		}
		// create control room
		c, err := JoinRoom(*room, *password)
		if err != nil {
			log.Fatal("failed to join room. ", *room)
		}
		defer c.Close()

		for {
			msg := make(map[string]interface{})
			err = c.ReadJSON(&msg)
			if err != nil {
				log.Println("failed to read JSON. ", err)
				continue
			}
			log.Printf("got message %v\n", msg)
			if msg["type"] == "createTunnel" {
				createTunnel(c, msg["destination"].(string), msg["refid"].(string))
			}
		}*/
}

func StartRelay(roomConn *websocket.Conn, tcpConn net.Conn, doClose func()) { /*
		go func() {
			for {
				_, p, err := roomConn.ReadMessage()
				if err != nil {
					log.Println("failed to read data from room. Closing tunnel")
					doClose()
					return
				}
				log.Printf("read %d bytes from room\n", len(p))
				n, err := tcpConn.Write(p)
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
			n, err := tcpConn.Read(buf)
			if err != nil {
				log.Println("failed to read data from tcp connection. Closing tunnel")
				doClose()
				return
			}
			log.Printf("read %d bytes from tcp connection\n", n)
			err = roomConn.WriteMessage(websocket.BinaryMessage, buf)
			if err != nil {
				log.Println("failed to write data to tunnel room. Closing tunnel")
				doClose()
				return
			}
			log.Printf("wrote %d bytes to room\n", n)
		}*/
}
func createTunnel(hubClient *hub.Client, controlRoom *hub.Room, destination, refid string) {
	tunnelRoom := uuid.New().String()
	tunnelPassword := uuid.New().String()

	returnFailure := func(cause string) {
		msg := make(map[string]interface{})
		msg["type"] = "tunnelCreationFailed"
		msg["destination"] = destination
		msg["refid"] = refid
		msg["cause"] = cause
		controlRoom.WriteJSON(msg)
	}
	var tcpConn net.Conn = nil
	roomConn, err := hubClient.Join(tunnelRoom, tunnelPassword)
	if err != nil {
		log.Println("failed to create room for tunnel")
		returnFailure("failed to create room for tunnel")
		return
	}
	doClose := func() {
		roomConn.Close()
		if tcpConn != nil {
			tcpConn.Close()
		}
		if *exitOnDisconnect {
			os.Exit(0)
		}
	}
	log.Println("opening connection to", destination)
	tcpAddr, err := net.ResolveTCPAddr("tcp", destination)
	if err != nil {
		log.Println("failed to resolve destination address", destination, err.Error())
		returnFailure("failed to resolve destination address")
		doClose()
		return
	}

	tcpConn, err = net.DialTCP("tcp", nil, tcpAddr)
	if err != nil {
		log.Println("Destination dial failed", tcpAddr, err.Error())
		returnFailure("failed to dial destination")
		doClose()
		return
	}
	msg := make(map[string]interface{})
	msg["type"] = "tunnelCreated"
	msg["destination"] = destination
	msg["refid"] = refid
	msg["room"] = tunnelRoom
	msg["password"] = tunnelPassword
	controlRoom.WriteJSON(msg)

	roomConn.Relay(tcpConn)
	if *exitOnDisconnect {
		os.Exit(0)
	}
}

func createTunnel_old(controlRoom *websocket.Conn, destination, refid string) {
	/*tunnelRoom := uuid.New().String()
	tunnelPassword := uuid.New().String()

	returnFailure := func(cause string) {
		msg := make(map[string]interface{})
		msg["type"] = "tunnelCreationFailed"
		msg["destination"] = destination
		msg["refid"] = refid
		msg["cause"] = cause
		controlRoom.WriteJSON(msg)
	}
	var tcpConn net.Conn = nil
	roomConn, err := JoinRoom(tunnelRoom, tunnelPassword)
	if err != nil {
		log.Println("failed to create room for tunnel")
		returnFailure("failed to create room for tunnel")
		return
	}
	doClose := func() {
		roomConn.Close()
		if tcpConn != nil {
			tcpConn.Close()
		}
		if *exitOnDisconnect {
			os.Exit(0)
		}
	}
	log.Println("opening connection to", destination)
	tcpAddr, err := net.ResolveTCPAddr("tcp", destination)
	if err != nil {
		log.Println("failed to resolve destination address", destination, err.Error())
		returnFailure("failed to resolve destination address")
		doClose()
		return
	}

	tcpConn, err = net.DialTCP("tcp", nil, tcpAddr)
	if err != nil {
		log.Println("Destination dial failed", tcpAddr, err.Error())
		returnFailure("failed to dial destination")
		doClose()
		return
	}
	msg := make(map[string]interface{})
	msg["type"] = "tunnelCreated"
	msg["destination"] = destination
	msg["refid"] = refid
	msg["room"] = tunnelRoom
	msg["password"] = tunnelPassword
	controlRoom.WriteJSON(msg)

	go StartRelay(roomConn, tcpConn, doClose)*/
}
