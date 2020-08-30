package main

import (
	"fmt"
	"log"
	"math/rand"
	"net"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hub/hub"
)

func startClient() {
	log.Println("client| starting client and connecting to", *client)
	hubClient := hub.NewClient(*client, *token, *bypassProxy)
	var controlRoom *hub.Room
	enterControlRoom := func() {
		var err error
		controlRoom, err = hubClient.Join(*room, *password)
		if err != nil {
			log.Fatal("client| failed to join room ", *room, err)
		}
	}
	enterControlRoom()
	defer func() {
		controlRoom.Close()
	}()
	destination := ""
	if len(*tunnel) > 0 {
		destination = *tunnel
	} else if len(*rdp) > 0 {
		destination = *rdp
		if strings.Index(destination, ":") == -1 {
			destination = destination + ":3389"
		}
	} else {
		log.Fatal("client| client must provide either -tunnel or -rdp arguments")
	}

	rand.Seed(time.Now().UnixNano())
	listenIf := fmt.Sprintf("127.%d.%d.%d:3389", rand.Intn(254), rand.Intn(254), 1+rand.Intn(253))
	if len(*listen) > 0 {
		listenIf = *listen
	}

	log.Println("client| listening for tcp/ip connection on", listenIf)
	listener, err := net.Listen("tcp", listenIf)
	if err != nil {
		log.Fatal("client| failed to listen on", listenIf, err)
	}

	handleConn := func(tcpConn net.Conn) {
		defer tcpConn.Close()
		log.Println("client| got connection from", tcpConn.RemoteAddr())
		refid := uuid.New().String()
		err := controlRoom.WriteJSON(&agentRequest{
			Type:        "createTunnel",
			Destination: destination,
			Refid:       refid})
		if err != nil {
			log.Println("client|", tcpConn.RemoteAddr(), "Failed to send createTunnel message.", err)
			if err.Error() == "websocket: close sent" {
				enterControlRoom()
			}
			return
		}
		var resp agentResponse
		err = controlRoom.ReadJSON(&resp)
		if err != nil {
			log.Println("client|", tcpConn.RemoteAddr(), "failed to read JSON. ", err)
			if err.Error() == "websocket: close 1006 (abnormal closure): unexpected EOF" {
				enterControlRoom()
			}
			return
		}
		if resp.Refid == refid {
			if resp.Type == "tunnelCreationFailed" {
				log.Println("client|", tcpConn.RemoteAddr(), "Tunnel creation failed. cause: %s", resp.Cause)
				return
			}
			log.Println("client|", tcpConn.RemoteAddr(), "Tunnel room created")
			tunnel, err := hubClient.Join(resp.Room, resp.Password)
			if err != nil {
				log.Println("client|", tcpConn.RemoteAddr(), "Failed to join tunnel room")
				return
			}
			log.Println("client|", tcpConn.RemoteAddr(), "Joined room", resp.Room, "Now relaying data with", listenIf)
			go func() {
				tunnel.Relay(tcpConn)
				if *exitOnDisconnect {
					os.Exit(0)
				}
			}()
		}

	}

	for {
		tcpConn, err := listener.Accept()
		if err != nil {
			log.Println("client| failed to accept connection.", err)
			continue
		}
		handleConn(tcpConn)
	}
}
