package broker

import "sync"

type SubscriptionTree struct {
	// Root
	root *TrieNode
	// Mutex
	mu sync.RWMutex
}

type TrieNode struct {
	children    map[string]*TrieNode
	subscribers map[string]*Subscriber // ClientID -> Subscriber
	isWildcard  bool                   // for + wildcards
	isMultiWild bool                   // for # wildcards
}

type Subscriber struct {
	Subscriber *Session
	Handler    func(topic string, payload []byte)
}

func NewSubscriptionTree() *SubscriptionTree {
	return &SubscriptionTree{
		root: &TrieNode{
			children:    make(map[string]*TrieNode),
			subscribers: make(map[string]*Subscriber),
		},
	}
}
