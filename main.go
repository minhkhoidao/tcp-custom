package main

import (
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"sync"
)

type Server struct {
	listenAddr string
	ln         net.Listener
	quitch     chan struct{}
	msgch      chan []int16
	wg         sync.WaitGroup
}

func NewServer(listenAddr string) *Server {
	return &Server{
		listenAddr: listenAddr,
		quitch:     make(chan struct{}),
		msgch:      make(chan []int16, 10),
	}
}

func (s *Server) Start() error {
	ln, err := net.Listen("tcp", s.listenAddr)
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}
	s.ln = ln
	defer s.ln.Close()

	go s.acceptClients()

	<-s.quitch
	close(s.msgch)
	s.wg.Wait()
	return nil
}

func (s *Server) acceptClients() {
	for {
		conn, err := s.ln.Accept()
		if err != nil {
			select {
			case <-s.quitch:
				return
			default:
				log.Printf("accept error: %v", err)
				continue
			}
		}
		log.Printf("new client connected: %s", conn.RemoteAddr())
		s.wg.Add(1)
		go s.handleClient(conn)
	}
}

func (s *Server) handleClient(conn net.Conn) {
	defer s.wg.Done()
	defer conn.Close()

	buf := make([]byte, 1024)
	for {
		n, err := conn.Read(buf)
		if err != nil {
			if err != net.ErrClosed {
				log.Printf("read error: %v", err)
			}
			return
		}

		int16Buffer := make([]int16, n/2)
		for i := 0; i < n/2; i++ {
			int16Buffer[i] = int16(binary.LittleEndian.Uint16(buf[i*2 : i*2+2]))
		}

		select {
		case s.msgch <- int16Buffer:
		case <-s.quitch:
			return
		}
	}
}

func (s *Server) requestToOtherServer() {
	conn, err := net.Dial("tcp", "localhost:8081")

	if err != nil {
		log.Printf("dial error: %v", err)
		return
	}
	// Read from the channel and send the data
	defer conn.Close()

	for data := range s.msgch {
		buf := make([]byte, len(data)*2)
		for i, v := range data {
			binary.LittleEndian.PutUint16(buf[i*2:], uint16(v))
		}
		_, err := conn.Write(buf)
		if err != nil {
			log.Printf("write error: %v", err)
			return
		}
	}
}

func main() {
	server := NewServer(":8080")
	log.Println("server is running on :8080")
	go server.requestToOtherServer()
	if err := server.Start(); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
