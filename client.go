package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/hub/hub"
	"github.com/tebeka/atexit"
)

type tunnelInfo struct {
	Listen, Destination string
}

type clientConfig struct {
	ModifyHostsFile bool
	Tunnels         []tunnelInfo
}

func readLines(filename string) ([]string, error) {
	dat, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	return strings.Split(string(dat), "\n"), nil
}

func writeLines(filename string, lines []string) error {
	body := strings.Join(lines, "\n")
	return ioutil.WriteFile(filename, []byte(body), 0644)
}

func cleanupHostsFileAtExit() {
	cleanHostsFile()
}

func cleanHostsFile() {
	log.Println("cleaning hosts file")
	lines, err := readLines(hostsFileName)
	if err != nil {
		log.Println("failed to read hosts file.", err)
		return
	}
	beginIndex := indexOf(lines, "# HUB BEGIN")
	endIndex := indexOf(lines, "# HUB END")
	if beginIndex == -1 {
		return
	}
	lines = append(lines[:beginIndex-1], lines[endIndex+1:]...)
	err = writeLines(hostsFileName, lines)
	if err != nil {
		log.Println("Failed to write hosts file.", err)
	}
}

func indexOf(lines []string, line string) int {
	for i, l := range lines {
		if l == line {
			return i
		}
	}
	return -1
}

func insert(a []string, c string, i int) []string {
	return append(a[:i], append([]string{c}, a[i:]...)...)
}

func addHostInHostsFile(ip, host string) {
	lines, err := readLines(hostsFileName)
	if err != nil {
		log.Println("failed to read hosts file.", err)
		return
	}
	toAdd := fmt.Sprintf("%s %s", ip, host)
	if indexOf(lines, toAdd) != -1 {
		return
	}
	log.Println("adding", toAdd, "into hosts file")
	beginIndex := indexOf(lines, "# HUB BEGIN")
	if beginIndex == -1 {
		beginIndex = len(lines)
		lines = append(lines, "# HUB BEGIN")
		lines = append(lines, "# HUB END")
	}
	lines = insert(lines, toAdd, beginIndex+1)
	err = writeLines(hostsFileName, lines)
	if err != nil {
		log.Println("Failed to write hosts file.", err)
	}
}

func getHost(hostPort string) string {
	hp := strings.Split(hostPort, ":")
	if len(hp) == 0 || len(hp[0]) == 0 {
		return "127.0.0.1"
	}
	return hp[0]
}

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

	createOneTunnel := func(listenIf, destination string) {
		log.Println("client| listening for tcp/ip connection on", listenIf)
		listener, err := net.Listen("tcp", listenIf)
		if err != nil {
			log.Fatal("client| failed to listen on", listenIf, err)
		}

		handleConn := func(tcpConn net.Conn) {
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
					defer tcpConn.Close()
					tunnel.Relay(tcpConn)
					if *exitOnDisconnect {
						atexit.Exit(0)
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

	createTunnels := func(cfg clientConfig) {
		wg := sync.WaitGroup{}
		if cfg.ModifyHostsFile {
			atexit.Register(cleanupHostsFileAtExit)
		}
		for _, info := range cfg.Tunnels {
			addHostInHostsFile(getHost(info.Listen), getHost(info.Destination))
			wg.Add(1)
			go func(info tunnelInfo) {
				createOneTunnel(info.Listen, info.Destination)
				wg.Done()
			}(info)
		}
		wg.Wait()
	}

	rand.Seed(time.Now().UnixNano())
	listenIf := fmt.Sprintf("127.%d.%d.%d:3389", rand.Intn(254), rand.Intn(254), 1+rand.Intn(253))
	if len(*listen) > 0 {
		listenIf = *listen
	}

	destination := ""
	if len(*tunnel) > 0 {
		destination = *tunnel
		createOneTunnel(listenIf, destination)
	} else if len(*rdp) > 0 {
		destination = *rdp
		if strings.Index(destination, ":") == -1 {
			destination = destination + ":3389"
		}
		createOneTunnel(listenIf, destination)
	} else if len(*tunnelsFile) > 0 {
		dat, err := ioutil.ReadFile(*tunnelsFile)
		if err != nil {
			log.Fatalf("failed to read JSON file %s. %s", *tunnelsFile, err)
		}
		var cfg clientConfig
		err = json.Unmarshal(dat, &cfg)
		if err != nil {
			log.Fatalf("failed to parse JSON from file %s. %s", *tunnelsFile, err)
		}
		createTunnels(cfg)
	} else {
		log.Fatal("client| client must provide either -tunnel or -rdp arguments")
	}

}
