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
	"time"

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

// New creates a new TCPServer instance
func New(addr string, db *sql.DB) *TCPServer {
	return &TCPServer{
		addr:           addr,
		broker:         broker.New(),
		maxConnections: 1000,
		authStore:      auth.NewStore(db),
	}
}

// Start begins accepting TCP connections
func (srv *TCPServer) Start(ctx context.Context) error {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%s", srv.addr))
	if err != nil {
		return err
	}
	srv.listener = listener
	go srv.accept(ctx)
	return nil
}

// Stop shuts down the listener gracefully
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

// Checks if the server can accept a new connection
func (srv *TCPServer) checkServerAvailability() string {
	if srv.isShuttingdown.Load() {
		return "server is shutting down"
	}
	if srv.currentConnections.Load() >= int32(srv.maxConnections) {
		return "maximum connections exceeded"
	}
	return ""
}

func (srv *TCPServer) handleConnection(conn net.Conn) {
	defer func() {
		conn.Close()
		srv.currentConnections.Add(-1)
		log.Printf("Connection from %s closed", conn.RemoteAddr())
	}()

	// Server load and shutdown checks
	if reason := srv.checkServerAvailability(); reason != "" {
		ack := pkt.NewConnAck(false, pkt.ServerUnavailable)
		conn.Write(ack)
		conn.Close()
		return
	}

	srv.currentConnections.Add(1)
	log.Printf("Client connected from %s (connections: %d/%d)", conn.RemoteAddr(), srv.currentConnections.Load(), srv.maxConnections)
	connectionTimestamp := time.Now().Unix()

	reader := bufio.NewReader(conn)
	sessionEstablished := false

	for {
		// Read fixed header (1 byte)
		fixedHeaderByte, err := reader.ReadByte()
		if err != nil {
			if err == io.EOF {
				log.Printf("Client %s disconnected", conn.RemoteAddr())
			} else {
				log.Printf("Read error from %s: %v", conn.RemoteAddr(), err)
			}
			return
		}

		// Read Remaining Length (variable-length int, max 4 bytes)
		remLenBuf := make([]byte, 4)
		remLenOffset := 0
		remainingLength := 0
		multiplier := 1

		for {
			if remLenOffset >= len(remLenBuf) {
				log.Printf("Remaining length too large from %s", conn.RemoteAddr())
				srv.sendAndClose(conn, pkt.NewConnAck(false, pkt.UnacceptableProtocolVersion))
				return
			}
			b, err := reader.ReadByte()
			if err != nil {
				log.Printf("Error reading remaining length from %s: %v", conn.RemoteAddr(), err)
				return
			}
			remLenBuf[remLenOffset] = b
			remLenOffset++
			remainingLength += int(b&0x7F) * multiplier
			multiplier *= 128
			if (b & 0x80) == 0 {
				break
			}
		}

		// Allocate full packet buffer (fixed header + remaining length + variable header/payload)
		totalPacketSize := 1 + remLenOffset + remainingLength
		rawPacket := make([]byte, totalPacketSize)
		rawPacket[0] = fixedHeaderByte
		copy(rawPacket[1:1+remLenOffset], remLenBuf[:remLenOffset])

		_, err = io.ReadFull(reader, rawPacket[1+remLenOffset:])
		if err != nil {
			log.Printf("Error reading full packet from %s: %v", conn.RemoteAddr(), err)
			return
		}

		packet, err := pkt.Parse(rawPacket)
		if err != nil {
			log.Printf("Parse error from %s: %v", conn.RemoteAddr(), err)

			var returnCode byte
			switch {
			case errors.Is(err, er.ErrUnsupportedProtocolLevel), errors.Is(err, er.ErrUnsupportedProtocolName):
				returnCode = pkt.UnacceptableProtocolVersion
			case errors.Is(err, er.ErrInvalidCharsClientID), errors.Is(err, er.ErrClientIDLengthExceed), errors.Is(err, er.ErrIdentifierRejected):
				returnCode = pkt.IdentifierRejected
			case errors.Is(err, er.ErrPasswordWithoutUsername), errors.Is(err, er.ErrMalformedUsernameField), errors.Is(err, er.ErrMalformedPasswordField):
				returnCode = pkt.BadUsernameOrPassword
			case errors.Is(err, er.ErrInvalidPacketLength):
				srv.sendAndClose(conn, pkt.NewConnAck(false, pkt.UnacceptableProtocolVersion))
				return
			default:
				srv.sendAndClose(conn, pkt.NewConnAck(false, pkt.ServerUnavailable))
				return
			}
			srv.sendAndClose(conn, pkt.NewConnAck(false, returnCode))
			return
		}

		if !sessionEstablished {
			if !packet.IsConnect() {
				log.Printf("Expected CONNECT from %s, got %v", conn.RemoteAddr(), packet.Type)
				srv.sendAndClose(conn, pkt.NewConnAck(false, pkt.UnacceptableProtocolVersion))
				return
			}
			session := packet.GetConnect()
			if session == nil {
				log.Printf("Invalid CONNECT packet from %s", conn.RemoteAddr())
				srv.sendAndClose(conn, pkt.NewConnAck(false, pkt.ServerUnavailable))
				return
			}

			// Auth check if username/password is provided
			if session.UsernameFlag && session.PasswordFlag {
				if err := srv.authStore.Authenticate(*session.Username, *session.Password); err != nil {
					log.Printf("Auth failed for %s: %v", session.ClientID, err)
					srv.sendAndClose(conn, pkt.NewConnAck(false, pkt.BadUsernameOrPassword))
					return
				}
			}

			// Session management: Clean or resume
			_, sessionExists := srv.broker.Get(session.ClientID)
			sessionPresent := false

			if session.CleanSession && sessionExists {
				log.Printf("Client %s requested clean session, deleting existing", session.ClientID)
				srv.broker.Delete(session.ClientID)
			} else if !session.CleanSession && sessionExists {
				log.Printf("Client %s resuming persistent session", session.ClientID)
				sessionPresent = true
			}

			// Store session
			srv.broker.Store(session.ClientID, &broker.Session{
				// Key Identifiers
				ClientID:     session.ClientID,
				CleanSession: session.CleanSession,

				// Will Flags
				WillTopic:   *session.WillTopic,
				WillMessage: *session.WillMessage,
				WillQoS:     session.WillQoS,
				WillRetain:  session.WillRetain,

				// Connection
				KeepAlive:           session.KeepAlive,
				Connected:           true,
				ConnectionTimestamp: connectionTimestamp,
				Conn:                conn,
			})

			// Send CONNACK
			conn.Write(pkt.NewConnAck(sessionPresent, pkt.ConnectionAccepted))
			sessionEstablished = true
			continue
		}

		switch packet.Type {
		case pkt.PUBLISH:
			p := packet.Publish
			if p == nil {
				log.Printf("Nil PUBLISH packet from %s", conn.RemoteAddr())
				return
			}
			log.Printf("Received PUBLISH: Topic=%s Payload=%s QoS=%d", p.Topic, string(p.Payload), p.QoS)

			if p.QoS == pkt.QoSAtLeastOnce && p.PacketID != nil {
				puback := pkt.NewPubAck(p)
				if _, err := conn.Write(puback.Encode()); err != nil {
					log.Printf("Error sending PUBACK to %s: %v", conn.RemoteAddr(), err)
					return
				}
				log.Printf("Sent PUBACK to %s for PacketID %d", conn.RemoteAddr(), *p.PacketID)
			}

		case pkt.SUBSCRIBE:
			suback := pkt.NewSubAck(packet.Subscribe)
			if _, err := conn.Write(suback.Encode()); err != nil {
				log.Printf("Error sending SUBACK to %s: %v", conn.RemoteAddr(), err)
				return
			}

		case pkt.UNSUBSCRIBE:
			unsuback := pkt.NewUnsubAck(packet.Unsubscribe)
			if _, err := conn.Write(unsuback.Encode()); err != nil {
				log.Printf("Error sending UNSUBACK to %s: %v", conn.RemoteAddr(), err)
				return
			}
			log.Printf("Sent UNSUBACK to %s for PacketID %d", conn.RemoteAddr(), packet.Unsubscribe.PacketID)

		case pkt.PINGREQ:
			pingresp := pkt.CreatePingresp()
			if _, err := conn.Write(pingresp.Encode()); err != nil {
				log.Printf("Error sending PINGRESP to %s: %v", conn.RemoteAddr(), err)
				return
			}
			log.Printf("Sent PINGRESP to %s", conn.RemoteAddr())

		case pkt.DISCONNECT:
			log.Printf("Received DISCONNECT from %s", conn.RemoteAddr())
			conn.Close()
			return

		default:
			log.Printf("Unhandled packet type %v from %s", packet.Type, conn.RemoteAddr())
			srv.sendAndClose(conn, pkt.NewConnAck(false, pkt.UnacceptableProtocolVersion))
			return
		}
	}
}

// sendAndClose sends an ACK (usually CONNACK) and closes the connection
func (srv *TCPServer) sendAndClose(conn net.Conn, ack []byte) {
	if len(ack) > 0 {
		if _, err := conn.Write(ack); err != nil {
			log.Printf("Error sending ACK to %s: %v", conn.RemoteAddr(), err)
		}
	}
	conn.Close()
}
