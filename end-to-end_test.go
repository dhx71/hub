package main

import (
	"log"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/hub/hub"
)

func setArgument(_client, _agent, _tunnel, _listen string, _dev, _bypassProxy bool) {
	client = &_client
	agent = &_agent
	tunnel = &_tunnel
	listen = &_listen
	dev = &_dev
	bypassProxy = &_bypassProxy
}

func Test_HubClientsServer(t *testing.T) {

	setArgument("ws://localhost:8080/hub", "ws://localhost:8080/hub", "127.0.0.1:7777", ":8080", true, true)
	go startServer()
	time.Sleep(time.Millisecond * 500)

	hubClient := hub.NewClient(*agent, *token, true)

	s1, err := hubClient.Join("my room", "my password")
	if err != nil {
		t.Errorf("failed to create room. %s", err)
	}

	_, err = hubClient.Join("my room", "my wrong password")
	if err == nil {
		t.Errorf("was able to join room with wrong password!")
	}

	s2, err := hubClient.Join("my room", "my password")
	if err != nil {
		t.Errorf("failed to join room. %s", err)
	}

	s3, err := hubClient.Join("my room", "my password")
	if err != nil {
		t.Errorf("failed to join room. %s", err)
	}

	err = s1.WriteJSON(struct{ Type string }{"salut!"})
	if err != nil {
		t.Errorf("failed to write json. %s", err)
	}

	msg2 := make(map[string]interface{})
	err = s2.ReadJSON(&msg2)
	if err != nil {
		t.Errorf("failed to read json. %s", err)
	}
	if msg2["Type"].(string) != "salut!" {
		t.Errorf("message not as expected. %v", msg2)

	}

	msg3 := make(map[string]interface{})
	err = s3.ReadJSON(&msg3)
	if err != nil {
		t.Errorf("failed to read json. %s", err)
	}
	if msg3["Type"].(string) != "salut!" {
		t.Errorf("message not as expected. %v", msg2)
	}

	err = s2.Close()
	if err != nil {
		t.Errorf("failed to close. %s", err)
	}
	err = s2.Close()
	if err == nil {
		t.Errorf("should be able to close one conn twice.")
	}

	err = s1.WriteJSON(struct{ Type string }{"hello!"})
	if err != nil {
		t.Errorf("failed to write json. %s", err)
	}

	msg2 = make(map[string]interface{})
	err = s2.ReadJSON(&msg2)
	if err == nil {
		t.Errorf("should not be able to read from close connection")
	}

	msg3 = make(map[string]interface{})
	err = s3.ReadJSON(&msg3)
	if err != nil {
		t.Errorf("failed to read json 3")
	}
}

func Test_Tunnel_EndToEnd(t *testing.T) {
	go func() {
		listener, _ := net.Listen("tcp", ":7777")
		conn, _ := listener.Accept()
		log.Println("Test_Tunnel_EndToEnd accepted connection on :7777")
		defer conn.Close()
		buf := make([]byte, 1500)
		n, err := conn.Read(buf)
		if n != 14 || string(buf[:n]) != "THIS IS A TEST" || err != nil {
			t.Errorf("listener didn't received expected message. n:%d, err:%v", n, err)
		}
		n, err = conn.Write([]byte("THIS IS A REPLY"))
		if err != nil || n != 15 {
			t.Errorf("listener could not send message. n:%d, err:%v", n, err)
		}
		//time.Sleep(time.Second)
	}()

	setArgument("ws://localhost:8080/hub", "ws://localhost:8080/hub", "127.0.0.1:7777", ":8080", true, true)
	go startServer()

	time.Sleep(time.Millisecond * 500)

	setArgument("ws://localhost:8080/hub", "ws://localhost:8080/hub", "127.0.0.1:7777", ":8888", true, true)
	go startAgent()
	time.Sleep(time.Millisecond * 500)
	go startClient()
	time.Sleep(time.Millisecond * 500)

	conn, err := net.Dial("tcp", "127.0.0.1:8888")
	if err != nil {
		t.Errorf("failed to dial tunnel. %s", err)
	}
	defer conn.Close()
	n, err := conn.Write([]byte("THIS IS A TEST"))
	if n != 14 || err != nil {
		t.Errorf("failed to send to tunnel n:%d err:%v", n, err)
	}
	buf := make([]byte, 1500)
	n, err = conn.Read(buf)
	if n != 15 || string(buf[:n]) != "THIS IS A REPLY" || err != nil {
		t.Errorf("tunnel didn't received expected message. n:%d, err:%v", n, err)
	}
	//time.Sleep(time.Second)
}

func Test_Multithreads_Tunnel_EndToEnd(t *testing.T) {
	handleConn := func(conn net.Conn) {
		buf := make([]byte, 1500)
		n, err := conn.Read(buf)
		if n != 14 || string(buf[:n]) != "THIS IS A TEST" || err != nil {
			t.Errorf("listener didn't received expected message. n:%d, err:%v", n, err)
		}
		n, err = conn.Write([]byte("THIS IS A REPLY"))
		if err != nil || n != 15 {
			t.Errorf("listener could not send message. n:%d, err:%v", n, err)
		}
		time.Sleep(time.Second)
		conn.Close()
	}
	go func() {
		listener, _ := net.Listen("tcp", ":7777")
		for {
			conn, _ := listener.Accept()
			log.Println("Test_Tunnel_EndToEnd accepted connection on :7777")
			go handleConn(conn)
		}
		//time.Sleep(time.Second)
	}()

	setArgument("ws://localhost:8080/hub", "ws://localhost:8080/hub", "127.0.0.1:7777", ":8080", true, true)
	go startServer()

	time.Sleep(time.Millisecond * 500)

	setArgument("ws://localhost:8080/hub", "ws://localhost:8080/hub", "127.0.0.1:7777", ":8888", true, true)
	go startAgent()
	time.Sleep(time.Millisecond * 500)
	go startClient()
	time.Sleep(time.Millisecond * 1500)

	log.Println("++++++++++++++++++++++++++++++++++++++++++++++++++++++++")
	wg := sync.WaitGroup{}
	simulateClient := func(instanceNb int) {
		conn, err := net.Dial("tcp", "127.0.0.1:8888")
		if err != nil {
			t.Errorf("tunnel client failed to dial. %s", err)
		}
		log.Println("Test_Multithreads_Tunnel_EndToEnd| dialed to", conn.RemoteAddr().String())
		n, err := conn.Write([]byte("THIS IS A TEST"))
		if n != 14 || err != nil {
			t.Errorf("tunnel client failed to send data n:%d err:%v", n, err)
		}
		log.Println("Test_Multithreads_Tunnel_EndToEnd| sent", n, "bytes to", conn.RemoteAddr().String())
		buf := make([]byte, 1500)
		n, err = conn.Read(buf)
		if n != 15 || string(buf[:n]) != "THIS IS A REPLY" || err != nil {
			t.Errorf("tunnel client didn't received expected message (routine #:%d). n:%d, err:%v", instanceNb, n, err)
		}
		log.Println("Test_Multithreads_Tunnel_EndToEnd| read", n, "bytes from", conn.RemoteAddr().String())
		log.Println("Test_Multithreads_Tunnel_EndToEnd| closing conn with", conn.RemoteAddr().String())
		conn.Close()
		wg.Done()
	}
	for i := 1; i <= 1; i++ {
		wg.Add(1)
		go simulateClient(i)
	}

	wg.Wait()
	time.Sleep(time.Second)
}
