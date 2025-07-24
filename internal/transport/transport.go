package transport

import (
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
