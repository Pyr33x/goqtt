package broker

import (
	"sync"
	"time"

	"github.com/pyr33x/goqtt/internal/packet"
)

// QoSManager handles QoS 1 and QoS 2 message flows
type QoSManager struct {
	pendingQoS1  map[string]map[uint16]*PendingMessage // clientID -> packetID -> message
	pendingQoS2  map[string]map[uint16]*PendingMessage // clientID -> packetID -> message
	qos2Received map[string]map[uint16]*ReceivedQoS2   // clientID -> packetID -> received message
	mu           sync.RWMutex
	retryTicker  *time.Ticker
	stopCh       chan struct{}
}

// PendingMessage represents a message waiting for acknowledgment
type PendingMessage struct {
	PacketID   uint16
	ClientID   string
	Topic      string
	Payload    []byte
	QoS        packet.QoSLevel
	Retain     bool
	Timestamp  time.Time
	RetryCount int
	MaxRetries int
	RetryDelay time.Duration
	Session    *Session
}

// ReceivedQoS2 represents a QoS 2 message in the middle of the handshake
type ReceivedQoS2 struct {
	PacketID  uint16
	ClientID  string
	Topic     string
	Payload   []byte
	Retain    bool
	Timestamp time.Time
}

const (
	DefaultMaxRetries = 3
	DefaultRetryDelay = 30 * time.Second
	QoS2Timeout       = 5 * time.Minute
)

// NewQoSManager creates a new QoS flow manager
func NewQoSManager() *QoSManager {
	qm := &QoSManager{
		pendingQoS1:  make(map[string]map[uint16]*PendingMessage),
		pendingQoS2:  make(map[string]map[uint16]*PendingMessage),
		qos2Received: make(map[string]map[uint16]*ReceivedQoS2),
		retryTicker:  time.NewTicker(10 * time.Second), // Check for retries every 10 seconds
		stopCh:       make(chan struct{}),
	}

	// Start retry goroutine
	go qm.retryLoop()

	return qm
}

// Stop shuts down the QoS manager
func (qm *QoSManager) Stop() {
	close(qm.stopCh)
	qm.retryTicker.Stop()
}

// AddPendingQoS1 adds a QoS 1 message waiting for PUBACK
func (qm *QoSManager) AddPendingQoS1(msg *PendingMessage) {
	qm.mu.Lock()
	defer qm.mu.Unlock()

	if qm.pendingQoS1[msg.ClientID] == nil {
		qm.pendingQoS1[msg.ClientID] = make(map[uint16]*PendingMessage)
	}

	msg.Timestamp = time.Now()
	msg.MaxRetries = DefaultMaxRetries
	msg.RetryDelay = DefaultRetryDelay
	qm.pendingQoS1[msg.ClientID][msg.PacketID] = msg
}

// AddPendingQoS2 adds a QoS 2 message waiting for PUBREC
func (qm *QoSManager) AddPendingQoS2(msg *PendingMessage) {
	qm.mu.Lock()
	defer qm.mu.Unlock()

	if qm.pendingQoS2[msg.ClientID] == nil {
		qm.pendingQoS2[msg.ClientID] = make(map[uint16]*PendingMessage)
	}

	msg.Timestamp = time.Now()
	msg.MaxRetries = DefaultMaxRetries
	msg.RetryDelay = DefaultRetryDelay
	qm.pendingQoS2[msg.ClientID][msg.PacketID] = msg
}

// HandlePubAck processes a PUBACK packet for QoS 1 flow
func (qm *QoSManager) HandlePubAck(clientID string, packetID uint16) bool {
	qm.mu.Lock()
	defer qm.mu.Unlock()

	if clientMessages, exists := qm.pendingQoS1[clientID]; exists {
		if _, exists := clientMessages[packetID]; exists {
			delete(clientMessages, packetID)
			if len(clientMessages) == 0 {
				delete(qm.pendingQoS1, clientID)
			}
			return true
		}
	}
	return false
}

// HandlePubRec processes a PUBREC packet for QoS 2 flow
func (qm *QoSManager) HandlePubRec(clientID string, packetID uint16) (*packet.PubrelPacket, bool) {
	qm.mu.Lock()
	defer qm.mu.Unlock()

	if clientMessages, exists := qm.pendingQoS2[clientID]; exists {
		if msg, exists := clientMessages[packetID]; exists {
			// Move from pending publish to pending pubrel
			delete(clientMessages, packetID)
			if len(clientMessages) == 0 {
				delete(qm.pendingQoS2, clientID)
			}

			// Create PUBREL packet
			pubrel := &packet.PubrelPacket{
				PacketID: packetID,
			}

			// Store the message for potential pubcomp handling
			if qm.qos2Received[clientID] == nil {
				qm.qos2Received[clientID] = make(map[uint16]*ReceivedQoS2)
			}

			qm.qos2Received[clientID][packetID] = &ReceivedQoS2{
				PacketID:  packetID,
				ClientID:  clientID,
				Topic:     msg.Topic,
				Payload:   msg.Payload,
				Retain:    msg.Retain,
				Timestamp: time.Now(),
			}

			return pubrel, true
		}
	}
	return nil, false
}

// HandlePubComp processes a PUBCOMP packet for QoS 2 flow
func (qm *QoSManager) HandlePubComp(clientID string, packetID uint16) bool {
	qm.mu.Lock()
	defer qm.mu.Unlock()

	if clientMessages, exists := qm.qos2Received[clientID]; exists {
		if _, exists := clientMessages[packetID]; exists {
			delete(clientMessages, packetID)
			if len(clientMessages) == 0 {
				delete(qm.qos2Received, clientID)
			}
			return true
		}
	}
	return false
}

// HandleIncomingQoS2Publish handles an incoming QoS 2 PUBLISH packet
func (qm *QoSManager) HandleIncomingQoS2Publish(clientID string, packetID uint16, topic string, payload []byte, retain bool) *packet.PubrecPacket {
	qm.mu.Lock()
	defer qm.mu.Unlock()

	// Check if we already received this packet (duplicate)
	if clientMessages, exists := qm.qos2Received[clientID]; exists {
		if _, exists := clientMessages[packetID]; exists {
			// Duplicate - just send PUBREC again
			return &packet.PubrecPacket{PacketID: packetID}
		}
	}

	// Store the received message
	if qm.qos2Received[clientID] == nil {
		qm.qos2Received[clientID] = make(map[uint16]*ReceivedQoS2)
	}

	qm.qos2Received[clientID][packetID] = &ReceivedQoS2{
		PacketID:  packetID,
		ClientID:  clientID,
		Topic:     topic,
		Payload:   payload,
		Retain:    retain,
		Timestamp: time.Now(),
	}

	return &packet.PubrecPacket{PacketID: packetID}
}

// HandleIncomingPubRel handles an incoming PUBREL packet
func (qm *QoSManager) HandleIncomingPubRel(clientID string, packetID uint16) (*ReceivedQoS2, *packet.PubcompPacket) {
	qm.mu.Lock()
	defer qm.mu.Unlock()

	if clientMessages, exists := qm.qos2Received[clientID]; exists {
		if msg, exists := clientMessages[packetID]; exists {
			// Return the message for delivery and create PUBCOMP
			pubcomp := &packet.PubcompPacket{PacketID: packetID}

			// Remove from received messages
			delete(clientMessages, packetID)
			if len(clientMessages) == 0 {
				delete(qm.qos2Received, clientID)
			}

			return msg, pubcomp
		}
	}

	// If we don't have the message, still send PUBCOMP (MQTT spec requirement)
	return nil, &packet.PubcompPacket{PacketID: packetID}
}

// CleanupClient removes all pending messages for a disconnected client
func (qm *QoSManager) CleanupClient(clientID string) {
	qm.mu.Lock()
	defer qm.mu.Unlock()

	delete(qm.pendingQoS1, clientID)
	delete(qm.pendingQoS2, clientID)
	delete(qm.qos2Received, clientID)
}

// GetPendingMessageCount returns the number of pending messages for a client
func (qm *QoSManager) GetPendingMessageCount(clientID string) (int, int) {
	qm.mu.RLock()
	defer qm.mu.RUnlock()

	qos1Count := 0
	qos2Count := 0

	if qos1Messages, exists := qm.pendingQoS1[clientID]; exists {
		qos1Count = len(qos1Messages)
	}

	if qos2Messages, exists := qm.pendingQoS2[clientID]; exists {
		qos2Count = len(qos2Messages)
	}

	return qos1Count, qos2Count
}

// retryLoop handles message retries and timeouts
func (qm *QoSManager) retryLoop() {
	for {
		select {
		case <-qm.stopCh:
			return
		case <-qm.retryTicker.C:
			qm.processRetries()
			qm.cleanupTimedOutMessages()
		}
	}
}

// processRetries handles retry logic for pending messages
func (qm *QoSManager) processRetries() {
	qm.mu.Lock()
	defer qm.mu.Unlock()

	now := time.Now()

	// Process QoS 1 retries
	for clientID, clientMessages := range qm.pendingQoS1 {
		for packetID, msg := range clientMessages {
			if now.Sub(msg.Timestamp) >= msg.RetryDelay {
				if msg.RetryCount < msg.MaxRetries {
					msg.RetryCount++
					msg.Timestamp = now
					qm.retryMessage(msg)
				} else {
					// Max retries reached, remove message
					delete(clientMessages, packetID)
					if len(clientMessages) == 0 {
						delete(qm.pendingQoS1, clientID)
					}
				}
			}
		}
	}

	// Process QoS 2 retries
	for clientID, clientMessages := range qm.pendingQoS2 {
		for packetID, msg := range clientMessages {
			if now.Sub(msg.Timestamp) >= msg.RetryDelay {
				if msg.RetryCount < msg.MaxRetries {
					msg.RetryCount++
					msg.Timestamp = now
					qm.retryMessage(msg)
				} else {
					// Max retries reached, remove message
					delete(clientMessages, packetID)
					if len(clientMessages) == 0 {
						delete(qm.pendingQoS2, clientID)
					}
				}
			}
		}
	}
}

// retryMessage resends a message
func (qm *QoSManager) retryMessage(msg *PendingMessage) {
	if msg.Session == nil || msg.Session.Conn == nil {
		return
	}

	// Create PUBLISH packet for retry
	publishPacket := &packet.PublishPacket{
		Topic:    msg.Topic,
		Payload:  msg.Payload,
		QoS:      msg.QoS,
		Retain:   msg.Retain,
		PacketID: &msg.PacketID,
		DUP:      true, // Set DUP flag for retries
	}

	// Send the packet
	data := publishPacket.Encode()
	if data != nil {
		msg.Session.Conn.Write(data)
	}
}

// cleanupTimedOutMessages removes QoS 2 received messages that have timed out
func (qm *QoSManager) cleanupTimedOutMessages() {
	qm.mu.Lock()
	defer qm.mu.Unlock()

	now := time.Now()

	for clientID, clientMessages := range qm.qos2Received {
		for packetID, msg := range clientMessages {
			if now.Sub(msg.Timestamp) >= QoS2Timeout {
				delete(clientMessages, packetID)
				if len(clientMessages) == 0 {
					delete(qm.qos2Received, clientID)
				}
			}
		}
	}
}

// GetStatistics returns QoS manager statistics
func (qm *QoSManager) GetStatistics() map[string]any {
	qm.mu.RLock()
	defer qm.mu.RUnlock()

	totalQoS1Pending := make(map[string]int)
	totalQoS2Pending := make(map[string]int)
	totalQoS2Received := make(map[string]int)

	for clientID, messages := range qm.pendingQoS1 {
		totalQoS1Pending[clientID] = len(messages)
	}

	for clientID, messages := range qm.pendingQoS2 {
		totalQoS2Pending[clientID] = len(messages)
	}

	for clientID, messages := range qm.qos2Received {
		totalQoS2Received[clientID] = len(messages)
	}

	return map[string]any{
		"qos1_pending":  totalQoS1Pending,
		"qos2_pending":  totalQoS2Pending,
		"qos2_received": totalQoS2Received,
		"total_clients": len(qm.pendingQoS1) + len(qm.pendingQoS2) + len(qm.qos2Received),
	}
}
