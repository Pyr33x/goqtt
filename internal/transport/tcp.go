package transport

import (
	"bufio"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"sync/atomic"

	"github.com/pyr33x/goqtt/internal/auth"
	"github.com/pyr33x/goqtt/internal/broker"
	pkt "github.com/pyr33x/goqtt/internal/packet"
	"github.com/pyr33x/goqtt/pkg/er"
)

type TCPServer struct {
	addr               string
	listener           net.Listener
	broker             *broker.Broker
	isShuttingdown     atomic.Bool
	maxConnections     int
	currentConnections atomic.Int32
	authStore          *auth.Store
}

func New(addr string, db *sql.DB) *TCPServer {
	return &TCPServer{
		addr:           addr,
		broker:         broker.New(),
		maxConnections: 1000,
		authStore:      auth.NewStore(db),
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

func (srv *TCPServer) Stop() error {
	srv.isShuttingdown.Store(true)
	if srv.listener != nil {
		return srv.listener.Close()
	}
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
				// Check if we're shutting down
				if srv.isShuttingdown.Load() {
					return
				}
				log.Println("accept error: ", err)
				continue
			}
			go srv.handleConnection(conn)
		}
	}
}

func (srv *TCPServer) checkServerAvailability() string {
	// Check if server is shutting down
	if srv.isShuttingdown.Load() {
		return "server is shutting down"
	}

	// Check connection limits (resource exhaustion)
	if srv.currentConnections.Load() >= int32(srv.maxConnections) {
		return "maximum connections exceeded"
	}

	return "" // Server is available
}

func (srv *TCPServer) handleConnection(conn net.Conn) {
	defer func() {
		conn.Close()
		srv.currentConnections.Add(-1)
		log.Printf("Connection from %s closed", conn.RemoteAddr())
	}()

	// Check server availability BEFORE processing any packets
	if unavailableReason := srv.checkServerAvailability(); unavailableReason != "" {
		ack := pkt.NewConnAck(false, pkt.ServerUnavailable)
		conn.Write(ack)
		conn.Close()
		return
	}

	srv.currentConnections.Add(1)
	log.Printf("Client connected from %s (connections: %d/%d)",
		conn.RemoteAddr(), srv.currentConnections.Load(), srv.maxConnections)

	reader := bufio.NewReader(conn)
	buf := make([]byte, 1024)
	sessionEstablished := false

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

		raw := buf[:n]
		packet, err := pkt.Parse(raw)
		if err != nil {
			log.Printf("Parse error from %s: %v", conn.RemoteAddr(), err)

			var returnCode byte
			switch {
			case errors.Is(err, er.ErrUnsupportedProtocolLevel) || errors.Is(err, er.ErrUnsupportedProtocolName):
				returnCode = pkt.UnacceptableProtocolVersion
			case errors.Is(err, er.ErrInvalidCharsClientID) || errors.Is(err, er.ErrClientIDLengthExceed) || errors.Is(err, er.ErrIdentifierRejected):
				returnCode = pkt.IdentifierRejected
			case errors.Is(err, er.ErrPasswordWithoutUsername) || errors.Is(err, er.ErrMalformedUsernameField) || errors.Is(err, er.ErrMalformedPasswordField):
				returnCode = pkt.BadUsernameOrPassword
			}

			ack := pkt.NewConnAck(false, returnCode)
			conn.Write(ack)
			conn.Close()

			return
		}

		// Handle first packet - must be CONNECT
		if !sessionEstablished {
			if !packet.IsConnect() {
				log.Printf("Expected CONNECT packet from %s, got packet type %v", conn.RemoteAddr(), packet.Type)
				ack := pkt.NewConnAck(false, pkt.UnacceptableProtocolVersion)
				conn.Write(ack)
				conn.Close()
				return
			}

			session := packet.GetConnect()
			if session == nil {
				log.Printf("Failed to get CONNECT data from %s", conn.RemoteAddr())
				ack := pkt.NewConnAck(false, pkt.ServerUnavailable)
				conn.Write(ack)
				conn.Close()
				return
			}

			// If username/password flags are true, then we do a validation over db
			if session.UsernameFlag && session.PasswordFlag {
				err := srv.authStore.Authenticate(session.Username, session.Password)
				if err != nil {
					log.Printf("Failed to authenticate %s: %v", session.ClientID, err)
					ack := pkt.NewConnAck(false, pkt.BadUsernameOrPassword)
					conn.Write(ack)
					conn.Close()
					return
				}
			}

			// Check if session already exists (for session present flag)
			_, sessionExists := srv.broker.Get(session.ClientID)

			// Determine session present flag
			sessionPresent := false
			if session.CleanSession && sessionExists {
				log.Printf("Clean session requested for Client ID '%s', deleting existing session", session.ClientID)
				srv.broker.Delete(session.ClientID)
			}

			if !session.CleanSession && sessionExists {
				log.Printf("Restoring existing persistent session for Client ID '%s'", session.ClientID)
				sessionPresent = true
			}

			// Create/update session
			sessionStore := &broker.Session{
				ClientID:     session.ClientID,
				CleanSession: session.CleanSession,
				KeepAlive:    session.KeepAlive,
			}

			srv.broker.Store(session.ClientID, sessionStore)

			// Send successful CONNACK with session present flag
			ack := pkt.NewConnAck(sessionPresent, pkt.ConnectionAccepted)
			conn.Write(ack)
			sessionEstablished = true

			continue
		}
	}
}
