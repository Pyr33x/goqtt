package broker

import (
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/pyr33x/goqtt/internal/logger"
	"github.com/pyr33x/goqtt/internal/packet"
	"github.com/pyr33x/goqtt/internal/packet/utils"
)

type Broker struct {
	session       atomic.Value
	subscriptions *SubscriptionTree
	retainedMsgs  map[string]*RetainedMessage
	retainedMu    sync.RWMutex
	rwmu          sync.RWMutex
	packetIDSeq   uint32
	qosManager    *QoSManager
	logger        *logger.Logger
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
		qosManager:    NewQoSManager(),
		logger:        logger.NewMQTTLogger("broker"),
	}
	b.session.Store(make(sessionMap)) // Initialize empty session map
	return b
}

// HandleSubscribe processes a SUBSCRIBE packet and returns a SUBACK packet
func (b *Broker) HandleSubscribe(session *Session, subscribePacket *packet.SubscribePacket) *packet.SubackPacket {
	if subscribePacket == nil || session == nil {
		b.logger.Error("Invalid subscribe packet or session")
		return nil
	}

	returnCodes := make([]byte, len(subscribePacket.Filters))

	for i, filter := range subscribePacket.Filters {
		// Validate topic filter using comprehensive validation
		if err := utils.ValidateTopicFilter(filter.Topic); err != nil {
			b.logger.LogError(err, "Invalid topic filter",
				logger.ClientID(session.ClientID),
				logger.String("topic_filter", filter.Topic))
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
			b.logger.LogError(err, "Failed to add subscription",
				logger.ClientID(session.ClientID),
				logger.String("topic_filter", filter.Topic))
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

		b.logger.LogSubscription(session.ClientID, filter.Topic, int(grantedQoS), "subscribe")

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
		b.logger.Error("Invalid unsubscribe packet or session")
		return nil
	}

	for _, topicFilter := range unsubscribePacket.TopicFilters {
		err := b.subscriptions.Unsubscribe(session.ClientID, topicFilter)
		if err != nil {
			b.logger.LogError(err, "Failed to remove subscription",
				logger.ClientID(session.ClientID),
				logger.String("topic_filter", topicFilter))
		} else {
			b.logger.LogSubscription(session.ClientID, topicFilter, 0, "unsubscribe")
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

	// Validate topic name using comprehensive validation
	if err := utils.ValidateTopicName(publishPacket.Topic); err != nil {
		return fmt.Errorf("invalid topic name: %s, error: %v", publishPacket.Topic, err)
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

	b.logger.LogPublish("", publishPacket.Topic, int(publishPacket.QoS), publishPacket.Retain, len(publishPacket.Payload))
	return nil
}

// HandleClientDisconnect removes all subscriptions for a disconnecting client
func (b *Broker) HandleClientDisconnect(clientID string) {
	b.subscriptions.UnsubscribeAll(clientID)
	b.qosManager.CleanupClient(clientID)
	b.logger.LogClientConnection(clientID, "", "disconnect")
}

// deliverMessage sends a message to a specific session with proper QoS flow handling
func (b *Broker) deliverMessage(session *Session, topic string, payload []byte, qos packet.QoSLevel, retain bool) {
	if session == nil || session.Conn == nil {
		b.logger.Error("Cannot deliver message: invalid session or connection")
		return
	}

	// Create PUBLISH packet for delivery
	publishPacket := &packet.PublishPacket{
		Topic:   topic,
		Payload: payload,
		QoS:     qos,
		Retain:  retain,
	}

	// Handle different QoS levels
	switch qos {
	case packet.QoSAtMostOnce:
		// QoS 0: Fire and forget
		b.sendPacket(session, publishPacket)

	case packet.QoSAtLeastOnce:
		// QoS 1: Wait for PUBACK
		packetID := b.generatePacketID()
		publishPacket.PacketID = &packetID

		// Store for retry/acknowledgment handling
		pendingMsg := &PendingMessage{
			PacketID: packetID,
			ClientID: session.ClientID,
			Topic:    topic,
			Payload:  payload,
			QoS:      qos,
			Retain:   retain,
			Session:  session,
		}
		b.qosManager.AddPendingQoS1(pendingMsg)

		b.sendPacket(session, publishPacket)
		b.logger.LogQoSFlow(session.ClientID, packetID, int(qos), "PUBLISH_SENT")

	case packet.QoSExactlyOnce:
		// QoS 2: PUBLISH -> PUBREC -> PUBREL -> PUBCOMP
		packetID := b.generatePacketID()
		publishPacket.PacketID = &packetID

		// Store for retry/acknowledgment handling
		pendingMsg := &PendingMessage{
			PacketID: packetID,
			ClientID: session.ClientID,
			Topic:    topic,
			Payload:  payload,
			QoS:      qos,
			Retain:   retain,
			Session:  session,
		}
		b.qosManager.AddPendingQoS2(pendingMsg)

		b.sendPacket(session, publishPacket)
		b.logger.LogQoSFlow(session.ClientID, packetID, int(qos), "PUBLISH_SENT")
	}
}

// sendPacket sends a packet to a session
func (b *Broker) sendPacket(session *Session, publishPacket *packet.PublishPacket) {
	data := publishPacket.Encode()
	if data != nil {
		_, err := session.Conn.Write(data)
		if err != nil {
			b.logger.LogError(err, "Failed to deliver message to client",
				logger.ClientID(session.ClientID))
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
		b.logger.LogRetainedMessage(publishPacket.Topic, "removed", 0)
	} else {
		// Store retained message
		b.retainedMsgs[publishPacket.Topic] = &RetainedMessage{
			Topic:   publishPacket.Topic,
			Payload: publishPacket.Payload,
			QoS:     publishPacket.QoS,
		}
		b.logger.LogRetainedMessage(publishPacket.Topic, "stored", len(publishPacket.Payload))
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

// HandlePubAck processes a PUBACK packet for QoS 1 flow
func (b *Broker) HandlePubAck(clientID string, packetID uint16) bool {
	success := b.qosManager.HandlePubAck(clientID, packetID)
	if success {
		b.logger.LogQoSFlow(clientID, packetID, 1, "PUBACK_RECEIVED")
	}
	return success
}

// HandlePubRec processes a PUBREC packet for QoS 2 flow
func (b *Broker) HandlePubRec(clientID string, packetID uint16) *packet.PubrelPacket {
	pubrel, success := b.qosManager.HandlePubRec(clientID, packetID)
	if success {
		b.logger.LogQoSFlow(clientID, packetID, 2, "PUBREC_RECEIVED")
	}
	return pubrel
}

// HandlePubComp processes a PUBCOMP packet for QoS 2 flow
func (b *Broker) HandlePubComp(clientID string, packetID uint16) bool {
	success := b.qosManager.HandlePubComp(clientID, packetID)
	if success {
		b.logger.LogQoSFlow(clientID, packetID, 2, "PUBCOMP_RECEIVED")
	}
	return success
}

// HandleIncomingQoS2Publish handles an incoming QoS 2 PUBLISH packet
func (b *Broker) HandleIncomingQoS2Publish(clientID string, packetID uint16, topic string, payload []byte, retain bool) *packet.PubrecPacket {
	pubrec := b.qosManager.HandleIncomingQoS2Publish(clientID, packetID, topic, payload, retain)
	b.logger.LogQoSFlow(clientID, packetID, 2, "PUBREC_SENT")
	return pubrec
}

// HandleIncomingPubRel handles an incoming PUBREL packet
func (b *Broker) HandleIncomingPubRel(clientID string, packetID uint16) (*packet.PubcompPacket, error) {
	receivedMsg, pubcomp := b.qosManager.HandleIncomingPubRel(clientID, packetID)

	// If we have the message, deliver it now
	if receivedMsg != nil {
		// Process the message through the broker
		publishPacket := &packet.PublishPacket{
			Topic:   receivedMsg.Topic,
			Payload: receivedMsg.Payload,
			QoS:     packet.QoSExactlyOnce,
			Retain:  receivedMsg.Retain,
		}

		if err := b.HandlePublish(publishPacket); err != nil {
			return pubcomp, err
		}
	}

	b.logger.LogQoSFlow(clientID, packetID, 2, "PUBCOMP_SENT")
	return pubcomp, nil
}

// Stop shuts down the broker and cleanup resources
func (b *Broker) Stop() {
	if b.qosManager != nil {
		b.qosManager.Stop()
	}
}
