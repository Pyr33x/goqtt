package broker

import (
	"fmt"
	"log"
	"sync"
	"sync/atomic"

	"github.com/pyr33x/goqtt/internal/packet"
)

type Broker struct {
	session       atomic.Value
	subscriptions *SubscriptionTree
	retainedMsgs  map[string]*RetainedMessage
	retainedMu    sync.RWMutex
	rwmu          sync.RWMutex
	packetIDSeq   uint32
}

type RetainedMessage struct {
	Topic   string
	Payload []byte
	QoS     packet.QoSLevel
}

func New() *Broker {
	b := &Broker{
		subscriptions: NewSubscriptionTree(),
		retainedMsgs:  make(map[string]*RetainedMessage),
	}
	b.session.Store(make(sessionMap)) // Initialize empty session map
	return b
}

// HandleSubscribe processes a SUBSCRIBE packet and returns a SUBACK packet
func (b *Broker) HandleSubscribe(session *Session, subscribePacket *packet.SubscribePacket) *packet.SubackPacket {
	if subscribePacket == nil || session == nil {
		log.Printf("Invalid subscribe packet or session")
		return nil
	}

	returnCodes := make([]byte, len(subscribePacket.Filters))

	for i, filter := range subscribePacket.Filters {
		// Validate topic filter
		if !IsValidTopicFilter(filter.Topic) {
			log.Printf("Invalid topic filter: %s", filter.Topic)
			returnCodes[i] = packet.SubackFailure
			continue
		}

		// Create subscription handler
		handler := func(topic string, payload []byte, qos packet.QoSLevel, retain bool) {
			b.deliverMessage(session, topic, payload, qos, retain)
		}

		// Add subscription to the tree
		err := b.subscriptions.Subscribe(session.ClientID, session, filter.Topic, filter.QoS, handler)
		if err != nil {
			log.Printf("Failed to add subscription for client %s, topic %s: %v", session.ClientID, filter.Topic, err)
			returnCodes[i] = packet.SubackFailure
			continue
		}

		// Grant the requested QoS level (or downgrade if needed)
		grantedQoS := b.getGrantedQoS(filter.QoS)
		switch grantedQoS {
		case packet.QoSAtMostOnce:
			returnCodes[i] = packet.SubackMaxQoS0
		case packet.QoSAtLeastOnce:
			returnCodes[i] = packet.SubackMaxQoS1
		case packet.QoSExactlyOnce:
			returnCodes[i] = packet.SubackMaxQoS2
		default:
			returnCodes[i] = packet.SubackFailure
		}

		log.Printf("Client %s subscribed to %s with QoS %d", session.ClientID, filter.Topic, grantedQoS)

		// Send retained messages that match this subscription
		b.sendRetainedMessages(session, filter.Topic, grantedQoS)
	}

	return &packet.SubackPacket{
		PacketID:    subscribePacket.PacketID,
		ReturnCodes: returnCodes,
	}
}

// HandleUnsubscribe processes an UNSUBSCRIBE packet and returns an UNSUBACK packet
func (b *Broker) HandleUnsubscribe(session *Session, unsubscribePacket *packet.UnsubscribePacket) *packet.UnsubackPacket {
	if unsubscribePacket == nil || session == nil {
		log.Printf("Invalid unsubscribe packet or session")
		return nil
	}

	for _, topicFilter := range unsubscribePacket.TopicFilters {
		err := b.subscriptions.Unsubscribe(session.ClientID, topicFilter)
		if err != nil {
			log.Printf("Failed to remove subscription for client %s, topic %s: %v", session.ClientID, topicFilter, err)
		} else {
			log.Printf("Client %s unsubscribed from %s", session.ClientID, topicFilter)
		}
	}

	return &packet.UnsubackPacket{
		PacketID: unsubscribePacket.PacketID,
	}
}

// HandlePublish processes a PUBLISH packet and delivers it to matching subscribers
func (b *Broker) HandlePublish(publishPacket *packet.PublishPacket) error {
	if publishPacket == nil {
		return fmt.Errorf("invalid publish packet")
	}

	// Validate topic name
	if !IsValidTopicName(publishPacket.Topic) {
		return fmt.Errorf("invalid topic name: %s", publishPacket.Topic)
	}

	// Handle retained messages
	if publishPacket.Retain {
		b.handleRetainedMessage(publishPacket)
	}

	// Find matching subscriptions
	matches := b.subscriptions.Match(publishPacket.Topic)

	// Deliver message to each matching subscriber
	for _, subscription := range matches {
		if subscription.Handler != nil {
			// Use the minimum QoS between published message and subscription
			deliveryQoS := minQoS(publishPacket.QoS, subscription.QoS)
			subscription.Handler(publishPacket.Topic, publishPacket.Payload, deliveryQoS, publishPacket.Retain)
		}
	}

	log.Printf("Published message to topic %s, delivered to %d subscribers", publishPacket.Topic, len(matches))
	return nil
}

// HandleClientDisconnect removes all subscriptions for a disconnecting client
func (b *Broker) HandleClientDisconnect(clientID string) {
	b.subscriptions.UnsubscribeAll(clientID)
	log.Printf("Removed all subscriptions for disconnected client: %s", clientID)
}

// deliverMessage sends a message to a specific session
func (b *Broker) deliverMessage(session *Session, topic string, payload []byte, qos packet.QoSLevel, retain bool) {
	if session == nil || session.Conn == nil {
		log.Printf("Cannot deliver message: invalid session or connection")
		return
	}

	// Create PUBLISH packet for delivery
	publishPacket := &packet.PublishPacket{
		Topic:   topic,
		Payload: payload,
		QoS:     qos,
		Retain:  retain,
	}

	// Set packet ID for QoS 1 and 2
	if qos > packet.QoSAtMostOnce {
		packetID := b.generatePacketID()
		publishPacket.PacketID = &packetID
	}

	// Encode and send the packet
	data := publishPacket.Encode()
	if data != nil {
		_, err := session.Conn.Write(data)
		if err != nil {
			log.Printf("Failed to deliver message to client %s: %v", session.ClientID, err)
		}
	}
}

// handleRetainedMessage stores or removes retained messages
func (b *Broker) handleRetainedMessage(publishPacket *packet.PublishPacket) {
	b.retainedMu.Lock()
	defer b.retainedMu.Unlock()

	if len(publishPacket.Payload) == 0 {
		// Empty payload removes retained message
		delete(b.retainedMsgs, publishPacket.Topic)
		log.Printf("Removed retained message for topic: %s", publishPacket.Topic)
	} else {
		// Store retained message
		b.retainedMsgs[publishPacket.Topic] = &RetainedMessage{
			Topic:   publishPacket.Topic,
			Payload: publishPacket.Payload,
			QoS:     publishPacket.QoS,
		}
		log.Printf("Stored retained message for topic: %s", publishPacket.Topic)
	}
}

// sendRetainedMessages sends retained messages that match a topic filter to a subscriber
func (b *Broker) sendRetainedMessages(session *Session, topicFilter string, maxQoS packet.QoSLevel) {
	b.retainedMu.RLock()
	defer b.retainedMu.RUnlock()

	for topic, retainedMsg := range b.retainedMsgs {
		if TopicMatches(topicFilter, topic) {
			// Use minimum QoS between retained message and subscription
			deliveryQoS := minQoS(retainedMsg.QoS, maxQoS)
			b.deliverMessage(session, topic, retainedMsg.Payload, deliveryQoS, true)
		}
	}
}

// getGrantedQoS returns the QoS level granted by the broker (could implement downgrading logic)
func (b *Broker) getGrantedQoS(requestedQoS packet.QoSLevel) packet.QoSLevel {
	// For now, grant the requested QoS level
	// In a production implementation, you might want to downgrade based on server policy
	if requestedQoS > packet.QoSExactlyOnce {
		return packet.QoSExactlyOnce
	}
	return requestedQoS
}

// generatePacketID generates a unique packet ID for QoS 1 and 2 messages
func (b *Broker) generatePacketID() uint16 {
	// Simple atomic counter for packet IDs (1-65535, wrapping around)
	id := atomic.AddUint32(&b.packetIDSeq, 1)
	if id == 0 {
		id = atomic.AddUint32(&b.packetIDSeq, 1) // Skip 0
	}
	return uint16(id)
}

// minQoS returns the minimum QoS level between two QoS levels
func minQoS(qos1, qos2 packet.QoSLevel) packet.QoSLevel {
	if qos1 < qos2 {
		return qos1
	}
	return qos2
}

// GetClientSubscriptions returns all subscriptions for a specific client
func (b *Broker) GetClientSubscriptions(clientID string) []*Subscription {
	return b.subscriptions.GetSubscriptions(clientID)
}

// GetSubscriptionCount returns the total number of active subscriptions
func (b *Broker) GetSubscriptionCount() int {
	// This would require traversing the tree to count all subscriptions
	// Implementation depends on whether you want to maintain a counter or compute on demand
	return 0 // Placeholder
}

// GetRetainedMessageCount returns the number of retained messages
func (b *Broker) GetRetainedMessageCount() int {
	b.retainedMu.RLock()
	defer b.retainedMu.RUnlock()
	return len(b.retainedMsgs)
}
