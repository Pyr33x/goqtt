package broker

import (
	"maps"
	"net"
)

type Session struct {
	ClientID     string
	CleanSession bool
	KeepAlive    uint16
	Conn         net.Conn
}

type sessionMap map[string]Session

func (b *Broker) Store(key string, session *Session) {
	b.rwmu.Lock()
	defer b.rwmu.Unlock()

	current := b.session.Load().(sessionMap)
	updated := make(sessionMap)
	maps.Copy(updated, current)
	updated[key] = *session

	b.session.Store(updated)
}

func (b *Broker) Get(key string) (*Session, bool) {
	current := b.session.Load().(sessionMap)
	val, ok := current[key]
	return &val, ok
}

func (b *Broker) Delete(key string) {
	b.rwmu.Lock()
	defer b.rwmu.Unlock()

	current := b.session.Load().(sessionMap)
	updated := make(sessionMap)
	maps.Copy(updated, current)
	delete(updated, key)

	b.session.Store(updated)
}
