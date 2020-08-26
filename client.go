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
	log.Println("starting client and connecting to", *client)
	hubClient := hub.NewClient(*client, *token, *bypassProxy)
	controlRoom, err := hubClient.Join(*room, *password)
	if err != nil {
		log.Fatal("failed to join room ", *room)
	}
	defer controlRoom.Close()
	destination := ""
	if len(*tunnel) > 0 {
		destination = *tunnel
	} else if len(*rdp) > 0 {
		destination = *rdp
		if strings.Index(destination, ":") == -1 {
			destination = destination + ":3389"
		}
	} else {
		log.Fatal("client must provide either -tunnel or -rdp arguments")
	}

	rand.Seed(time.Now().UnixNano())
	listenIf := fmt.Sprintf("127.%d.%d.%d:3389", rand.Intn(254), rand.Intn(254), 1+rand.Intn(253))
	if len(*listen) > 0 {
		listenIf = *listen
	}

	log.Println("listening for tcp/ip connection on", listenIf)
	listener, err := net.Listen("tcp", listenIf)
	if err != nil {
		log.Fatal("failed to listen on", listenIf, err)
	}
	for {
		tcpConn, err := listener.Accept()
		if err != nil {
			log.Println("failed to accept connection.", err)
			continue
		}
		go func(tcpConn net.Conn) {
			defer tcpConn.Close()
			msg := make(map[string]interface{})
			msg["type"] = "createTunnel"
			msg["destination"] = destination
			refid := uuid.New().String()
			msg["refid"] = refid
			err := controlRoom.WriteJSON(msg)
			if err != nil {
				log.Println("Failed to send createRoom message")
				return
			}
			msg, err = controlRoom.ReadJSON()
			if err != nil {
				log.Println("failed to read JSON. ", err)
				return
			}
			if msg["refid"] == refid {
				if msg["type"] == "tunnelCreationFailed" {
					log.Println("Tunnel creation failed. cause: %s", msg["cause"].(string))
					return
				}
				log.Println("Tunnel room created")
				tunnelRoom := msg["room"].(string)
				tunnelPassword := msg["password"].(string)
				tunnel, err := hubClient.Join(tunnelRoom, tunnelPassword)
				if err != nil {
					log.Println("Failed to join tunnel room")
					return
				}
				log.Println("Joined room", tunnelRoom, "Now relaying data with", listenIf)
				defer tunnel.Close()
				tunnel.Relay(tcpConn)
				if *exitOnDisconnect {
					os.Exit(0)
				}
			}
		}(tcpConn)
	}
}
