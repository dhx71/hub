package main

import (
	"log"
	"net"
	"os"

	"github.com/google/uuid"
	"github.com/hub/hub"
)

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
