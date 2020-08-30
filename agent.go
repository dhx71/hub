package main

import (
	"log"
	"net"
	"os"

	"github.com/dhx71/hub/hublib"
"github.com/google/uuid"
	"github.com/tebeka/atexit"
)

type agentRequest struct {
	Type, Destination, Refid string
}

type agentResponse struct {
	Type, Room, Password, Refid, Cause string
	Success                            bool
}

func startAgent() {
	hubClient := hublib.NewClient(*agent, *token, *bypassProxy)
	controlRoom, err := hubClient.Join(*room, *password)
	if err != nil {
		log.Fatal("agent | failed to join room. ", *room)
	}
	defer controlRoom.Close()
	for {
		var req agentRequest
		err := controlRoom.ReadJSON(&req)
		if err != nil {
			log.Println("agent | failed to read JSON from room", *room, err)
			if err.Error() == "websocket: close 1006 (abnormal closure): unexpected EOF" {
				controlRoom, err = hubClient.Join(*room, *password)
				if err != nil {
					log.Fatal("agent | failed to join room. ", *room)
				}
			}
			continue
		}
		log.Printf("agent | got message on room %s: %v\n", *room, req)
		if req.Type == "createTunnel" {
			createTunnel(hubClient, controlRoom, req.Destination, req.Refid)
		}
	}

}

func createTunnel(hubClient *hublib.Client, controlRoom *hublib.Room, destination, refid string) {
	tunnelRoom := uuid.New().String()
	tunnelPassword := uuid.New().String()

	returnFailure := func(cause string) {
		controlRoom.WriteJSON(agentResponse{
			Type:    "tunnelCreationFailed",
			Refid:   refid,
			Cause:   cause,
			Success: false})
	}
	var tcpConn net.Conn = nil
	roomConn, err := hubClient.Join(tunnelRoom, tunnelPassword)
	if err != nil {
		log.Println(tunnelRoom, "agent | failed to create room for tunnel")
		returnFailure("failed to create room for tunnel")
		return
	}
	doClose := func() {
		roomConn.Close()
		if tcpConn != nil {
			tcpConn.Close()
		}
		if *exitOnDisconnect {
			atexit.Exit(0)
		}
	}
	log.Println("agent |", tunnelRoom, "opening connection to", destination)
	tcpAddr, err := net.ResolveTCPAddr("tcp", destination)
	if err != nil {
		log.Println("agent | failed to resolve destination address", destination, err.Error())
		returnFailure("failed to resolve destination address")
		doClose()
		return
	}

	tcpConn, err = net.DialTCP("tcp", nil, tcpAddr)
	if err != nil {
		log.Println("agent |", tunnelRoom, "Destination dial failed", tcpAddr, err.Error())
		returnFailure("failed to dial destination")
		doClose()
		return
	}
	err = controlRoom.WriteJSON(agentResponse{
		Type:     "tunnelCreated",
		Refid:    refid,
		Room:     tunnelRoom,
		Password: tunnelPassword})
	if err != nil {
		log.Println("agent |", tunnelRoom, "Failed to send tunnelCreated message back to client")
		doClose()
		return
	}

	go func() {
		roomConn.Relay(tcpConn)
		if *exitOnDisconnect {
			os.Exit(0)
		}
	}()
}
