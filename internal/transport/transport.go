package transport

import "github.com/pyr33x/goqtt/internal/broker"

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
