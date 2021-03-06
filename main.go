package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/dhx71/hub/hublib"
	"github.com/tebeka/atexit"
	"golang.org/x/crypto/acme/autocert"
)

var (
	domain           = flag.String("domain", "", "https domain to whitelist in certificate")
	token            = flag.String("token", "Please don't mention my secret phrase!", "token to provide when connecting to websocket server.\nSame token must be used by hub, agent and client.")
	dev              = flag.Bool("dev", false, "to listen in plain http on port 8080 without Let's Encrypt certificate")
	listen           = flag.String("listen", ":https", "listening host:port")
	agent            = flag.String("agent", "", "start hub as an agent and connect to spefified hub. Ex.: wss://10.0.0.3/hub/")
	password         = flag.String("password", "my room password", "specifies a password that the agent requires from clients")
	client           = flag.String("client", "", "start hub as a client and connect to spefified hub. Ex.: wss://10.0.0.3/hub/")
	room             = flag.String("room", "control room", "room used by client and agent to allow client to send command to agent")
	tunnel           = flag.String("tunnel", "", "creates a tunnel from this computer (-listen) to agent. This parameter contains host:port to tunnel to. Must be used with -client and -listen arguments")
	tunnelsFile      = flag.String("tunnels", "", "creates many tunnels as specified in JSON file. See above for an example.")
	rdp              = flag.String("rdp", "", "creates a tunnel from this computer to agent on RDP port. This parameter contains host to tunnel to. Must be used with -client argument. It will autonatically start mstsc.exe")
	bypassProxy      = flag.Bool("bypass-proxy", false, "bypass system proxy")
	proxy = flag.String("proxy", "", "specifies proxy URL")
	exitOnDisconnect = flag.Bool("exit-on-disconnect", false, "Stops the client when the tcp connection on the tunnel disconnects")
	exitAfter        = flag.Duration("exit-after", 0, "tells the application to terminate automatically after the given duration (ex.: 1h30m)")
)

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage of hub:

Run central websocket hub as follow. Certificate for provided domain automatically 
requested from Let's Encrypt unless argument -dev is used.

	hub -domain www.mydomain.io -token "secret"
	
Run agent instance to run command on behalf of client.

	hub -agent wss://www.mydomain.io/hub -token "secret" -room "room" -password "password"
	
Run a client to tunnel tcp/ip traffic over. Client listens on -listen

    hub -client wss://www.mydomain.io/hub -token "secret" -room "room" -password "password" -tunnel 192.168.2.4:3389 -listen 127.0.0.1:8888

Run a client to tunnel tcp/ip RDP traffic over. Will start mstsc.exe.

    hub -client wss://www.mydomain.io/hub -token "secret" -room "room" -password "password" -rdp 192.168.2.4

Run a client to tunnel multiple tcp/ip connections traffic over from many ports.

	hub -client wss://www.mydomain.io/hub -token "secret" -room "room" -password "password" -tunnels tunnels.json
	
JSON file looks like:
{
	"ModifyHostsFile":true, // need admin rights
	"Tunnels":[{
		"Listen":"127.0.0.2:3389",
		"Destination": "server1.lab.mycompany.net:3389"
	}, {
		"Listen":"127.0.0.2:8080",
		"Destination": "server1.lab.mycompany.net:8080"
	}, {
		"Listen":"127.0.0.3:443",
		"Destination": "server2.lab.mycompany.net:443"
	}]
}
`)

		flag.PrintDefaults()
	}
	flag.Parse()
	setupCloseHandler()

	if *exitAfter != 0 {
		go func() {
			<-time.After(*exitAfter)
			log.Println("running for", *exitAfter, "time to exit!")
			atexit.Exit(1)
		}()
	}

	if len(*agent) > 0 {
		startAgent()
		return
	}
	if len(*client) > 0 {
		startClient()
		return
	}
	startServer()
}

func setupCloseHandler() {
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		fmt.Println("Ctrl+C pressed in Terminal")
		atexit.Exit(0)
	}()
}

func startServer() {
	_ = os.Mkdir("./webapps", os.ModeDir)
	http.Handle("/", http.FileServer(http.Dir("./webapps")))
	http.HandleFunc("/hub", hublib.NewHubHandlerFunc(*token))
	if !*dev {
		_ = os.Mkdir("./secret-dir", os.ModeDir)
		m := &autocert.Manager{
			Cache:      autocert.DirCache("./secret-dir"),
			Prompt:     autocert.AcceptTOS,
			HostPolicy: autocert.HostWhitelist(*domain),
		}
		s := &http.Server{
			Addr:      *listen,
			TLSConfig: m.TLSConfig(),
		}
		s.ListenAndServeTLS("", "")
	} else {
		s := &http.Server{
			Addr: *listen,
		}
		s.ListenAndServe()
	}
}
