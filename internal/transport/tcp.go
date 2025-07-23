package transport

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"strings"

	"github.com/pyr33x/goqtt/internal/broker"
)

type TCPServer struct {
	addr   string
	broker *broker.Broker
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
	defer listener.Close()

	go func() {
		<-ctx.Done()
		listener.Close()
	}()

	for {
		conn, err := listener.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				log.Println("TCP server shutting down...")
				return nil
			default:
				log.Printf("failed to accept connection: %v", err)
				continue
			}
		}
		go srv.connect(ctx, conn)
	}
}

func (srv *TCPServer) connect(ctx context.Context, conn net.Conn) {
	defer conn.Close()
	reader := bufio.NewReader(conn)

	log.Printf("Client connected from %s\n", conn.RemoteAddr())

	for {
		select {
		case <-ctx.Done():
			log.Printf("Closing connection with %s due to server shutdown", conn.RemoteAddr())
			return
		default:
			packet, err := reader.ReadBytes('\n')
			if err == io.EOF {
				log.Printf("Client %s disconnected", conn.RemoteAddr())
				return
			}
			if err != nil {
				log.Printf("Read error from %s: %v", conn.RemoteAddr(), err)
				return
			}

			srv.handlePacket(strings.TrimSpace(string(packet)), conn)
		}
	}
}

func (srv *TCPServer) handlePacket(packet string, conn net.Conn) {
	log.Printf("Received: %s", packet)
	fmt.Fprintf(conn, "ACK: %s", packet)
}
