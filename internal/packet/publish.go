package packet

type QoSLevel uint8

const (
	QoSAtMostOnce  QoSLevel = 0 // QoS 0
	QoSAtLeastOnce QoSLevel = 1 // QoS 1
	QoSExactlyOnce QoSLevel = 2 // QoS 2
)

type PublishPacket struct {
	// Fixed Header
	DUP    bool
	QoS    QoSLevel
	Retain bool

	// Variable Header
	Topic    string
	PacketID *uint16 // nil for QoS 0, pointer to ID for QoS 1/2

	// Payload
	Payload []byte

	// Raw
	Raw []byte
}
