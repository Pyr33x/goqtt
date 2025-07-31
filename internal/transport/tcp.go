package transport

import (
	"bufio"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"net"
	"sync/atomic"
	"time"

	"github.com/pyr33x/goqtt/internal/auth"
	"github.com/pyr33x/goqtt/internal/broker"
	"github.com/pyr33x/goqtt/internal/logger"
	er "github.com/pyr33x/goqtt/pkg/er"

	pkt "github.com/pyr33x/goqtt/internal/packet"
)

type TCPServer struct {
	addr               string
	listener           net.Listener
	broker             *broker.Broker
	isShuttingdown     atomic.Bool
	maxConnections     int
	currentConnections atomic.Int32
	authStore          *auth.Store
	logger             *logger.Logger
}

// New creates a new TCPServer instance
func New(addr string, db *sql.DB) *TCPServer {
	return &TCPServer{
		addr:           addr,
		broker:         broker.New(),
		maxConnections: 1000,
		authStore:      auth.NewStore(db),
		logger:         logger.NewMQTTLogger("tcp-server"),
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
			srv.logger.Info("shutting down accept...")
			return
		default:
			conn, err := srv.listener.Accept()
			if err != nil {
				if srv.isShuttingdown.Load() {
					return
				}
				srv.logger.LogError(err, "accept error")
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
	var clientID string
	defer func() {
		conn.Close()
		srv.currentConnections.Add(-1)

		// Clean up subscriptions when connection closes
		if clientID != "" {
			srv.broker.HandleClientDisconnect(clientID)
		}

		srv.logger.LogClientConnection("", conn.RemoteAddr().String(), "closed")
	}()

	// Server load and shutdown checks
	if reason := srv.checkServerAvailability(); reason != "" {
		ack := pkt.NewConnAck(false, pkt.ServerUnavailable)
		conn.Write(ack)
		conn.Close()
		return
	}

	srv.currentConnections.Add(1)
	srv.logger.LogClientConnection("", conn.RemoteAddr().String(), "connected",
		logger.Int("current_connections", int(srv.currentConnections.Load())),
		logger.Int("max_connections", int(srv.maxConnections)))

	reader := bufio.NewReader(conn)
	sessionEstablished := false

	for {
		// Read fixed header (1 byte)
		fixedHeaderByte, err := reader.ReadByte()
		if err != nil {
			if err == io.EOF {
				srv.logger.LogClientConnection("", conn.RemoteAddr().String(), "disconnected")
			} else {
				srv.logger.LogError(err, "Read error", logger.String("remote_addr", conn.RemoteAddr().String()))
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
				srv.logger.Error("Remaining length too large", logger.String("remote_addr", conn.RemoteAddr().String()))
				srv.sendAndClose(conn, pkt.NewConnAck(false, pkt.UnacceptableProtocolVersion))
				return
			}
			b, err := reader.ReadByte()
			if err != nil {
				srv.logger.LogError(err, "Error reading remaining length", logger.String("remote_addr", conn.RemoteAddr().String()))
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
			srv.logger.LogError(err, "Error reading full packet", logger.String("remote_addr", conn.RemoteAddr().String()))
			return
		}

		packet, err := pkt.Parse(rawPacket)
		if err != nil {
			srv.logger.LogError(err, "Parse error", logger.String("remote_addr", conn.RemoteAddr().String()))

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
				srv.logger.Error("Expected CONNECT packet",
					logger.String("remote_addr", conn.RemoteAddr().String()),
					logger.String("got_packet_type", packet.Type.String()))
				srv.sendAndClose(conn, pkt.NewConnAck(false, pkt.UnacceptableProtocolVersion))
				return
			}
			session := packet.GetConnect()
			if session == nil {
				srv.logger.Error("Invalid CONNECT packet", logger.String("remote_addr", conn.RemoteAddr().String()))
				srv.sendAndClose(conn, pkt.NewConnAck(false, pkt.ServerUnavailable))
				return
			}

			// Auth check if username/password is provided
			if session.UsernameFlag && session.PasswordFlag {
				if err := srv.authStore.Authenticate(*session.Username, *session.Password); err != nil {
					srv.logger.LogAuth(session.ClientID, *session.Username, false, "authentication failed")
					srv.sendAndClose(conn, pkt.NewConnAck(false, pkt.BadUsernameOrPassword))
					return
				}
			}

			// Session management: Clean or resume
			_, sessionExists := srv.broker.Get(session.ClientID)
			sessionPresent := false

			if session.CleanSession && sessionExists {
				srv.logger.LogClientConnection(session.ClientID, conn.RemoteAddr().String(), "clean_session_requested")
				srv.broker.Delete(session.ClientID)
			} else if !session.CleanSession && sessionExists {
				srv.logger.LogClientConnection(session.ClientID, conn.RemoteAddr().String(), "persistent_session_resumed")
				sessionPresent = true
			}

			// Send CONNACK
			conn.Write(pkt.NewConnAck(sessionPresent, pkt.ConnectionAccepted))
			sessionEstablished = true

			// Store session
			brokerSession := &broker.Session{
				// Key Identifiers
				ClientID:     session.ClientID,
				CleanSession: session.CleanSession,

				// Will Flags
				WillTopic:   session.WillTopic,
				WillMessage: session.WillMessage,
				WillQoS:     session.WillQoS,
				WillRetain:  session.WillRetain,

				// Connection
				KeepAlive:           session.KeepAlive,
				ConnectionTimestamp: time.Now().Unix(),
				Conn:                conn,
			}
			srv.broker.Store(session.ClientID, brokerSession)
			clientID = session.ClientID // Store for cleanup
			continue
		}

		// Get current session for packet handling
		currentSession, exists := srv.broker.Get(clientID)
		if !exists {
			// Check if packet type can be handled without a session
			if packet.Type == pkt.DISCONNECT {
				srv.logger.LogClientConnection("", conn.RemoteAddr().String(), "disconnect_without_session")
				conn.Close()
				return
			}
			srv.logger.Error("Session not found for connection", logger.String("remote_addr", conn.RemoteAddr().String()))
			return
		}

		switch packet.Type {
		case pkt.PUBLISH:
			p := packet.Publish
			if p == nil {
				srv.logger.Error("Nil PUBLISH packet", logger.String("remote_addr", conn.RemoteAddr().String()))
				return
			}
			srv.logger.LogPublish(currentSession.ClientID, p.Topic, int(p.QoS), p.Retain, len(p.Payload))

			// Handle different QoS levels for incoming PUBLISH
			switch p.QoS {
			case pkt.QoSAtMostOnce:
				// QoS 0: Just process the message
				if err := srv.broker.HandlePublish(currentSession.ClientID, p); err != nil {
					srv.logger.LogError(err, "Error handling PUBLISH", logger.ClientID(currentSession.ClientID))
				}

			case pkt.QoSAtLeastOnce:
				// QoS 1: Process and send PUBACK
				if p.PacketID == nil {
					srv.logger.Error("Missing PacketID for QoS 1", logger.ClientID(currentSession.ClientID))
					return
				}

				if err := srv.broker.HandlePublish(currentSession.ClientID, p); err != nil {
					srv.logger.LogError(err, "Error handling PUBLISH", logger.ClientID(currentSession.ClientID))
				}

				puback := pkt.NewPubAck(p)
				if _, err := conn.Write(puback.Encode()); err != nil {
					srv.logger.LogError(err, "Error sending PUBACK", logger.ClientID(currentSession.ClientID))
					return
				}
				srv.logger.LogQoSFlow(currentSession.ClientID, *p.PacketID, 1, "PUBACK_SENT")

			case pkt.QoSExactlyOnce:
				// QoS 2: Send PUBREC, wait for PUBREL
				if p.PacketID == nil {
					srv.logger.Error("Missing PacketID for QoS 2", logger.ClientID(currentSession.ClientID))
					return
				}

				pubrec := srv.broker.HandleIncomingQoS2Publish(currentSession.ClientID, *p.PacketID, p.Topic, p.Payload, p.Retain)
				if _, err := conn.Write(pubrec.Encode()); err != nil {
					srv.logger.LogError(err, "Error sending PUBREC", logger.ClientID(currentSession.ClientID))
					return
				}
				srv.logger.LogQoSFlow(currentSession.ClientID, *p.PacketID, 2, "PUBREC_SENT")
			}

		case pkt.PUBACK:
			if packet.Puback == nil {
				srv.logger.Error("Nil PUBACK packet", logger.String("remote_addr", conn.RemoteAddr().String()))
				return
			}
			srv.broker.HandlePubAck(currentSession.ClientID, packet.Puback.PacketID)

		case pkt.PUBREC:
			if packet.Pubrec == nil {
				srv.logger.Error("Nil PUBREC packet", logger.String("remote_addr", conn.RemoteAddr().String()))
				return
			}
			pubrel := srv.broker.HandlePubRec(currentSession.ClientID, packet.Pubrec.PacketID)
			if pubrel != nil {
				if _, err := conn.Write(pubrel.Encode()); err != nil {
					srv.logger.LogError(err, "Error sending PUBREL", logger.ClientID(currentSession.ClientID))
					return
				}
				srv.logger.LogQoSFlow(currentSession.ClientID, packet.Pubrec.PacketID, 2, "PUBREL_SENT")
			}

		case pkt.PUBREL:
			if packet.Pubrel == nil {
				srv.logger.Error("Nil PUBREL packet", logger.String("remote_addr", conn.RemoteAddr().String()))
				return
			}
			pubcomp, err := srv.broker.HandleIncomingPubRel(currentSession.ClientID, packet.Pubrel.PacketID)
			if err != nil {
				srv.logger.LogError(err, "Error handling PUBREL", logger.ClientID(currentSession.ClientID))
			}
			if pubcomp != nil {
				if _, err := conn.Write(pubcomp.Encode()); err != nil {
					srv.logger.LogError(err, "Error sending PUBCOMP", logger.ClientID(currentSession.ClientID))
					return
				}
				srv.logger.LogQoSFlow(currentSession.ClientID, packet.Pubrel.PacketID, 2, "PUBCOMP_SENT")
			}

		case pkt.PUBCOMP:
			if packet.Pubcomp == nil {
				srv.logger.Error("Nil PUBCOMP packet", logger.String("remote_addr", conn.RemoteAddr().String()))
				return
			}
			srv.broker.HandlePubComp(currentSession.ClientID, packet.Pubcomp.PacketID)

		case pkt.SUBSCRIBE:
			if packet.Subscribe == nil {
				srv.logger.Error("Nil SUBSCRIBE packet", logger.String("remote_addr", conn.RemoteAddr().String()))
				return
			}

			// Handle subscription through broker
			suback := srv.broker.HandleSubscribe(currentSession, packet.Subscribe)
			if suback == nil {
				srv.logger.Error("Failed to handle SUBSCRIBE", logger.String("remote_addr", conn.RemoteAddr().String()))
				return
			}

			// Send SUBACK response
			if _, err := conn.Write(suback.Encode()); err != nil {
				srv.logger.LogError(err, "Error sending SUBACK", logger.ClientID(currentSession.ClientID))
				return
			}
			srv.logger.LogMQTTPacket("SUBACK", currentSession.ClientID, "outbound", logger.Int("packet_id", int(suback.PacketID)))

		case pkt.UNSUBSCRIBE:
			if packet.Unsubscribe == nil {
				srv.logger.Error("Nil UNSUBSCRIBE packet", logger.String("remote_addr", conn.RemoteAddr().String()))
				return
			}

			// Handle unsubscription through broker
			unsuback := srv.broker.HandleUnsubscribe(currentSession, packet.Unsubscribe)
			if unsuback == nil {
				srv.logger.Error("Failed to handle UNSUBSCRIBE", logger.String("remote_addr", conn.RemoteAddr().String()))
				return
			}

			// Send UNSUBACK response
			if _, err := conn.Write(unsuback.Encode()); err != nil {
				srv.logger.LogError(err, "Error sending UNSUBACK", logger.ClientID(currentSession.ClientID))
				return
			}
			srv.logger.LogMQTTPacket("UNSUBACK", currentSession.ClientID, "outbound", logger.Int("packet_id", int(unsuback.PacketID)))

		case pkt.PINGREQ:
			pingresp := pkt.CreatePingresp()
			if _, err := conn.Write(pingresp.Encode()); err != nil {
				srv.logger.LogError(err, "Error sending PINGRESP", logger.ClientID(currentSession.ClientID))
				return
			}
			srv.logger.LogMQTTPacket("PINGRESP", currentSession.ClientID, "outbound")

		case pkt.SUBACK:
			if packet.Suback == nil {
				srv.logger.Error("Nil SUBACK packet", logger.String("remote_addr", conn.RemoteAddr().String()))
				return
			}
			srv.logger.LogMQTTPacket("SUBACK", currentSession.ClientID, "inbound", logger.Int("packet_id", int(packet.Suback.PacketID)))
			// TODO: Handle subscription acknowledgment (match with pending subscriptions)

		case pkt.UNSUBACK:
			if packet.Unsuback == nil {
				srv.logger.Error("Nil UNSUBACK packet", logger.String("remote_addr", conn.RemoteAddr().String()))
				return
			}
			srv.logger.LogMQTTPacket("UNSUBACK", currentSession.ClientID, "inbound", logger.Int("packet_id", int(packet.Unsuback.PacketID)))
			// TODO: Handle unsubscription acknowledgment (match with pending unsubscriptions)

		case pkt.DISCONNECT:
			srv.logger.LogClientConnection(currentSession.ClientID, conn.RemoteAddr().String(), "disconnect")

			// Clean up subscriptions for this client
			if currentSession != nil {
				srv.broker.HandleClientDisconnect(currentSession.ClientID)
			}

			conn.Close()
			return

		default:
			srv.logger.Error("Unhandled packet type",
				logger.String("packet_type", packet.Type.String()),
				logger.String("remote_addr", conn.RemoteAddr().String()))
			srv.sendAndClose(conn, pkt.NewConnAck(false, pkt.UnacceptableProtocolVersion))
			return
		}
	}
}

// sendAndClose sends an ACK (usually CONNACK) and closes the connection
func (srv *TCPServer) sendAndClose(conn net.Conn, ack []byte) {
	if len(ack) > 0 {
		if _, err := conn.Write(ack); err != nil {
			srv.logger.LogError(err, "Error sending ACK", logger.String("remote_addr", conn.RemoteAddr().String()))
		}
	}
	conn.Close()
}
