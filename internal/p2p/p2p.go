// Package p2p provides peer-to-peer networking capabilities for OpenFang.
package p2p

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"time"
)

// PeerID represents a unique peer identifier.
type PeerID string

// PeerInfo represents information about a peer.
type PeerInfo struct {
	ID        PeerID            `json:"id"`
	Name      string            `json:"name"`
	Addresses []string          `json:"addresses"`
	Version   string            `json:"version"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	Connected bool              `json:"connected"`
	LastSeen  *time.Time        `json:"last_seen,omitempty"`
}

// MessageType represents the type of a P2P message.
type MessageType string

const (
	MessageTypeHello      MessageType = "hello"
	MessageTypePing       MessageType = "ping"
	MessageTypePong       MessageType = "pong"
	MessageTypeAgentState MessageType = "agent_state"
	MessageTypeHandShare  MessageType = "hand_share"
	MessageTypeDataSync   MessageType = "data_sync"
)

// Message represents a P2P message.
type Message struct {
	Type      MessageType `json:"type"`
	From      PeerID      `json:"from"`
	To        PeerID      `json:"to,omitempty"`
	Timestamp time.Time   `json:"timestamp"`
	Payload   []byte      `json:"payload"`
}

// Connection represents a P2P connection to a peer.
type Connection struct {
	mu       sync.RWMutex
	peerID   PeerID
	conn     net.Conn
	ctx      context.Context
	cancel   context.CancelFunc
	sendCh   chan *Message
	recvCh   chan *Message
	closed   bool
}

// NetworkConfig represents the P2P network configuration.
type NetworkConfig struct {
	ListenAddr   string
	AnnounceAddr string
	BootstrapPeers []string
	EnableMDNS   bool
	EnableDHT    bool
}

// Network represents the P2P network manager.
type Network struct {
	mu          sync.RWMutex
	localID     PeerID
	localInfo   *PeerInfo
	config      *NetworkConfig
	peers       map[PeerID]*PeerInfo
	connections map[PeerID]*Connection
	listener    net.Listener
	ctx         context.Context
	cancel      context.CancelFunc
	messageCh   chan *Message
}

// NewNetwork creates a new P2P network.
func NewNetwork(localID PeerID, config *NetworkConfig) *Network {
	ctx, cancel := context.WithCancel(context.Background())

	if config == nil {
		config = &NetworkConfig{
			ListenAddr: ":0",
		}
	}

	localInfo := &PeerInfo{
		ID:      localID,
		Name:    fmt.Sprintf("fangclaw-%s", string(localID)[:8]),
		Version: "0.2.0",
	}

	return &Network{
		localID:     localID,
		localInfo:   localInfo,
		config:      config,
		peers:       make(map[PeerID]*PeerInfo),
		connections: make(map[PeerID]*Connection),
		ctx:         ctx,
		cancel:      cancel,
		messageCh:   make(chan *Message, 100),
	}
}

// Start starts the P2P network.
func (n *Network) Start() error {
	listener, err := net.Listen("tcp", n.config.ListenAddr)
	if err != nil {
		return err
	}
	n.listener = listener

	if n.config.AnnounceAddr == "" {
		addr := listener.Addr().String()
		n.config.AnnounceAddr = addr
		n.localInfo.Addresses = []string{addr}
	}

	go n.acceptLoop()

	for _, peerAddr := range n.config.BootstrapPeers {
		go n.connectToPeer(peerAddr)
	}

	if n.config.EnableMDNS {
		go n.mdnsDiscovery()
	}

	return nil
}

// Stop stops the P2P network.
func (n *Network) Stop() error {
	n.cancel()

	if n.listener != nil {
		n.listener.Close()
	}

	n.mu.Lock()
	defer n.mu.Unlock()

	for _, conn := range n.connections {
		conn.Close()
	}

	return nil
}

// acceptLoop accepts incoming connections.
func (n *Network) acceptLoop() {
	for {
		select {
		case <-n.ctx.Done():
			return
		default:
			conn, err := n.listener.Accept()
			if err != nil {
				select {
				case <-n.ctx.Done():
					return
				default:
					fmt.Printf("Error accepting connection: %v\n", err)
					continue
				}
			}
			go n.handleConnection(conn)
		}
	}
}

// handleConnection handles an incoming connection.
func (n *Network) handleConnection(conn net.Conn) {
	peerConn := n.newConnection("", conn)
	peerConn.Start()
}

// connectToPeer connects to a peer.
func (n *Network) connectToPeer(addr string) {
	select {
	case <-n.ctx.Done():
		return
	default:
	}

	dialer := net.Dialer{Timeout: 10 * time.Second}
	conn, err := dialer.DialContext(n.ctx, "tcp", addr)
	if err != nil {
		fmt.Printf("Failed to connect to %s: %v\n", addr, err)
		return
	}

	peerConn := n.newConnection("", conn)
	peerConn.Start()

	helloMsg := &Message{
		Type:      MessageTypeHello,
		From:      n.localID,
		Timestamp: time.Now(),
	}
	
	payload, _ := json.Marshal(n.localInfo)
	helloMsg.Payload = payload
	
	peerConn.Send(helloMsg)
}

// newConnection creates a new connection.
func (n *Network) newConnection(peerID PeerID, conn net.Conn) *Connection {
	ctx, cancel := context.WithCancel(n.ctx)
	
	peerConn := &Connection{
		peerID: peerID,
		conn:   conn,
		ctx:    ctx,
		cancel: cancel,
		sendCh: make(chan *Message, 50),
		recvCh: make(chan *Message, 50),
	}

	return peerConn
}

// GetLocalInfo gets the local peer information.
func (n *Network) GetLocalInfo() *PeerInfo {
	return n.localInfo
}

// GetPeers gets all known peers.
func (n *Network) GetPeers() []*PeerInfo {
	n.mu.RLock()
	defer n.mu.RUnlock()

	peers := make([]*PeerInfo, 0, len(n.peers))
	for _, peer := range n.peers {
		peers = append(peers, peer)
	}
	return peers
}

// GetPeer gets a peer by ID.
func (n *Network) GetPeer(peerID PeerID) (*PeerInfo, bool) {
	n.mu.RLock()
	defer n.mu.RUnlock()
	peer, ok := n.peers[peerID]
	return peer, ok
}

// SendMessage sends a message to a peer.
func (n *Network) SendMessage(to PeerID, msg *Message) error {
	n.mu.RLock()
	conn, ok := n.connections[to]
	n.mu.RUnlock()

	if !ok {
		return fmt.Errorf("peer not connected: %s", to)
	}

	msg.From = n.localID
	msg.To = to
	msg.Timestamp = time.Now()

	return conn.Send(msg)
}

// BroadcastMessage broadcasts a message to all connected peers.
func (n *Network) BroadcastMessage(msg *Message) {
	msg.From = n.localID
	msg.Timestamp = time.Now()

	n.mu.RLock()
	connections := make([]*Connection, 0, len(n.connections))
	for _, conn := range n.connections {
		connections = append(connections, conn)
	}
	n.mu.RUnlock()

	for _, conn := range connections {
		conn.Send(msg)
	}
}

// Messages returns the message channel.
func (n *Network) Messages() <-chan *Message {
	return n.messageCh
}

// mdnsDiscovery performs mDNS discovery (placeholder).
func (n *Network) mdnsDiscovery() {
	select {
	case <-n.ctx.Done():
		return
	}
}

// Start starts the connection.
func (c *Connection) Start() {
	go c.readLoop()
	go c.writeLoop()
}

// Close closes the connection.
func (c *Connection) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil
	}

	c.closed = true
	c.cancel()
	close(c.sendCh)
	close(c.recvCh)
	
	return c.conn.Close()
}

// Send sends a message through the connection.
func (c *Connection) Send(msg *Message) error {
	select {
	case <-c.ctx.Done():
		return c.ctx.Err()
	case c.sendCh <- msg:
		return nil
	}
}

// readLoop reads messages from the connection.
func (c *Connection) readLoop() {
	decoder := json.NewDecoder(c.conn)
	
	for {
		select {
		case <-c.ctx.Done():
			return
		default:
			var msg Message
			if err := decoder.Decode(&msg); err != nil {
				fmt.Printf("Error reading message: %v\n", err)
				return
			}
			
			select {
			case c.recvCh <- &msg:
			case <-c.ctx.Done():
				return
			}
		}
	}
}

// writeLoop writes messages to the connection.
func (c *Connection) writeLoop() {
	encoder := json.NewEncoder(c.conn)
	
	for {
		select {
		case <-c.ctx.Done():
			return
		case msg := <-c.sendCh:
			if err := encoder.Encode(msg); err != nil {
				fmt.Printf("Error writing message: %v\n", err)
				return
			}
		}
	}
}

// GeneratePeerID generates a random peer ID.
func GeneratePeerID() PeerID {
	b := make([]byte, 16)
	for i := range b {
		b[i] = byte(time.Now().UnixNano() % 256)
	}
	return PeerID(fmt.Sprintf("%x", b))
}
