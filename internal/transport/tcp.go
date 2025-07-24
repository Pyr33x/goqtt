package transport

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"net"

	"github.com/pyr33x/goqtt/internal/broker"
)

type TCPServer struct {
	addr     string
	listener net.Listener
	broker   *broker.Broker
}

func New(addr string, broker *broker.Broker) *TCPServer {
	return &TCPServer{
		addr:   addr,
		broker: broker,
	}
}

func (srv *TCPServer) Start(ctx context.Context) error {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%s", srv.addr))
	if err != nil {
		return err
	}
	srv.listener = listener
	go srv.accept(ctx)
	return nil
}

func (srv *TCPServer) accept(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			log.Println("shutting down accept...")
			return
		default:
			conn, err := srv.listener.Accept()
			if err != nil {
				log.Println("accept error: ", err)
				continue
			}
			go srv.read(conn)
		}
	}
}

func (srv *TCPServer) read(conn net.Conn) {
	defer conn.Close()
	reader := bufio.NewReader(conn)
	log.Printf("Client connected from %s\n", conn.RemoteAddr())

	buf := make([]byte, 1024)
	for {
		n, err := reader.Read(buf)
		if err == io.EOF {
			log.Printf("Client %s disconnected", conn.RemoteAddr())
			return
		}
		if err != nil {
			log.Printf("Read error from %s: %v", conn.RemoteAddr(), err)
			return
		}
		buf = buf[:n]
	}
}
