package broker

import (
	"strings"
	"sync"

	"github.com/pyr33x/goqtt/internal/packet"
	"github.com/pyr33x/goqtt/internal/packet/utils"
)

type SubscriptionTree struct {
	root *TrieNode
	mu   sync.RWMutex
}

type TrieNode struct {
	children    map[string]*TrieNode
	subscribers map[string]*Subscription // ClientID -> Subscription
	isWildcard  bool                     // true if this node represents a wildcard
}

type Subscription struct {
	ClientID string
	Session  *Session
	QoS      packet.QoSLevel
	Handler  func(topic string, payload []byte, qos packet.QoSLevel, retain bool)
}

func NewSubscriptionTree() *SubscriptionTree {
	return &SubscriptionTree{
		root: &TrieNode{
			children:    make(map[string]*TrieNode),
			subscribers: make(map[string]*Subscription),
		},
	}
}

// Subscribe adds a subscription to the tree
func (st *SubscriptionTree) Subscribe(clientID string, session *Session, topicFilter string, qos packet.QoSLevel, handler func(string, []byte, packet.QoSLevel, bool)) error {
	// Add validation step at the start
	if err := utils.ValidateTopicFilter(topicFilter); err != nil {
		return err
	}

	st.mu.Lock()
	defer st.mu.Unlock()

	// Split topic filter into levels
	levels := strings.Split(topicFilter, "/")

	current := st.root

	// Navigate/create the path in the trie
	for _, level := range levels {
		if current.children == nil {
			current.children = make(map[string]*TrieNode)
		}

		if current.children[level] == nil {
			current.children[level] = &TrieNode{
				children:    make(map[string]*TrieNode),
				subscribers: make(map[string]*Subscription),
				isWildcard:  level == "+" || level == "#",
			}
		}

		current = current.children[level]

		// For multi-level wildcard (#), we stop here as it matches everything below
		if level == "#" {
			break
		}
	}

	// Add/update subscription at this node
	if current.subscribers == nil {
		current.subscribers = make(map[string]*Subscription)
	}

	current.subscribers[clientID] = &Subscription{
		ClientID: clientID,
		Session:  session,
		QoS:      qos,
		Handler:  handler,
	}

	return nil
}

// Unsubscribe removes a subscription from the tree
func (st *SubscriptionTree) Unsubscribe(clientID string, topicFilter string) error {
	st.mu.Lock()
	defer st.mu.Unlock()

	// Split topic filter into levels
	levels := strings.Split(topicFilter, "/")

	current := st.root
	path := make([]*TrieNode, 0, len(levels)+1)
	path = append(path, current)

	// Navigate to the subscription node
	for _, level := range levels {
		if current.children == nil || current.children[level] == nil {
			return nil // Subscription doesn't exist
		}

		current = current.children[level]
		path = append(path, current)

		if level == "#" {
			break
		}
	}

	// Remove the subscription
	if current.subscribers != nil {
		delete(current.subscribers, clientID)
	}

	// Clean up empty nodes from leaf to root
	st.cleanupEmptyNodes(path, levels)

	return nil
}

// UnsubscribeAll removes all subscriptions for a client
func (st *SubscriptionTree) UnsubscribeAll(clientID string) {
	st.mu.Lock()
	defer st.mu.Unlock()

	st.removeClientFromTree(st.root, clientID)
}

// removeClientFromTree recursively removes a client from all nodes
func (st *SubscriptionTree) removeClientFromTree(node *TrieNode, clientID string) {
	if node == nil {
		return
	}

	// Remove client from current node
	if node.subscribers != nil {
		delete(node.subscribers, clientID)
	}

	// Recursively remove from children
	for _, child := range node.children {
		st.removeClientFromTree(child, clientID)
	}
}

// cleanupEmptyNodes removes empty nodes from the tree
func (st *SubscriptionTree) cleanupEmptyNodes(path []*TrieNode, levels []string) {
	// Start from the leaf and work backwards
	for i := len(path) - 1; i > 0; i-- {
		node := path[i]
		parent := path[i-1]

		// If node has subscribers or children, keep it
		if len(node.subscribers) > 0 || len(node.children) > 0 {
			break
		}

		// Remove empty node from parent
		levelIndex := i - 1
		if levelIndex < len(levels) {
			level := levels[levelIndex]
			delete(parent.children, level)
		}
	}
}

// Match finds all subscriptions that match a given topic
func (st *SubscriptionTree) Match(topic string) []*Subscription {
	st.mu.RLock()
	defer st.mu.RUnlock()

	var matches []*Subscription
	topicLevels := strings.Split(topic, "/")

	st.matchRecursive(st.root, topicLevels, 0, &matches)

	return matches
}

// matchRecursive recursively matches topic levels against the subscription tree
func (st *SubscriptionTree) matchRecursive(node *TrieNode, topicLevels []string, levelIndex int, matches *[]*Subscription) {
	if node == nil {
		return
	}

	// If we've consumed all topic levels, collect subscribers from this node
	if levelIndex >= len(topicLevels) {
		for _, sub := range node.subscribers {
			*matches = append(*matches, sub)
		}
		return
	}

	currentLevel := topicLevels[levelIndex]

	// Check for exact match
	if exactChild, exists := node.children[currentLevel]; exists {
		st.matchRecursive(exactChild, topicLevels, levelIndex+1, matches)
	}

	// Check for single-level wildcard (+)
	if plusChild, exists := node.children["+"]; exists {
		st.matchRecursive(plusChild, topicLevels, levelIndex+1, matches)
	}

	// Check for multi-level wildcard (#)
	if hashChild, exists := node.children["#"]; exists {
		// Multi-level wildcard matches everything from this point
		for _, sub := range hashChild.subscribers {
			*matches = append(*matches, sub)
		}
	}
}

// GetSubscriptions returns all subscriptions for a specific client
func (st *SubscriptionTree) GetSubscriptions(clientID string) []*Subscription {
	st.mu.RLock()
	defer st.mu.RUnlock()

	var subscriptions []*Subscription
	st.getClientSubscriptions(st.root, clientID, &subscriptions)

	return subscriptions
}

// getClientSubscriptions recursively finds all subscriptions for a client
func (st *SubscriptionTree) getClientSubscriptions(node *TrieNode, clientID string, subscriptions *[]*Subscription) {
	if node == nil {
		return
	}

	// Check if client has subscription at this node
	if sub, exists := node.subscribers[clientID]; exists {
		*subscriptions = append(*subscriptions, sub)
	}

	// Recursively check children
	for _, child := range node.children {
		st.getClientSubscriptions(child, clientID, subscriptions)
	}
}

// IsValidTopicFilter validates a topic filter according to MQTT 3.1.1 rules
func IsValidTopicFilter(topicFilter string) bool {
	return utils.ValidateTopicFilter(topicFilter) == nil
}

// IsValidTopicName validates a topic name for publishing (no wildcards allowed)
func IsValidTopicName(topicName string) bool {
	return utils.ValidateTopicName(topicName) == nil
}

// TopicMatches checks if a topic name matches a topic filter
func TopicMatches(topicFilter, topicName string) bool {
	if !IsValidTopicFilter(topicFilter) || !IsValidTopicName(topicName) {
		return false
	}

	filterLevels := strings.Split(topicFilter, "/")
	nameLevels := strings.Split(topicName, "/")

	return topicMatchesRecursive(filterLevels, nameLevels, 0, 0)
}

// topicMatchesRecursive recursively matches topic filter levels against topic name levels
func topicMatchesRecursive(filterLevels, nameLevels []string, filterIndex, nameIndex int) bool {
	// If we've consumed all filter levels
	if filterIndex >= len(filterLevels) {
		return nameIndex >= len(nameLevels) // Must have consumed all name levels too
	}

	// If we've consumed all name levels but still have filter levels
	if nameIndex >= len(nameLevels) {
		return false
	}

	currentFilter := filterLevels[filterIndex]
	currentName := nameLevels[nameIndex]

	switch currentFilter {
	case "+":
		// Single-level wildcard matches exactly one level
		return topicMatchesRecursive(filterLevels, nameLevels, filterIndex+1, nameIndex+1)
	case "#":
		// Multi-level wildcard matches remaining levels
		return true
	default:
		// Exact match required
		if currentFilter == currentName {
			return topicMatchesRecursive(filterLevels, nameLevels, filterIndex+1, nameIndex+1)
		}
		return false
	}
}
