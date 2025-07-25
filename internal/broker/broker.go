package broker

import (
	"sync"
	"sync/atomic"
)

type Broker struct {
	session atomic.Value
	rwmu    sync.RWMutex
}

func New() *Broker {
	b := &Broker{}
	b.session.Store(make(sessionMap)) // Initialize empty session map
	return b
}
