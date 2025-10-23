package network

import (
	"log"
	"math/rand"
	"sync"
	"time"
)

// Network 模拟链路层 fabric（进程内交换/转发器）
type Network struct {
	nodes    map[uint32]*Node // 所有节点的映射表
	nodesMux sync.RWMutex     // 保护节点映射表的读写锁

	// 网络延迟和丢包模拟参数
	latency    time.Duration // 模拟网络延迟
	packetLoss float32       // 丢包率 (0.0 - 1.0)

	// 统计信息
	totalMessages   uint64       // 总发送消息数
	droppedMessages uint64       // 丢失消息数
	statsMux        sync.RWMutex // 统计信息锁
}

// deliverMessage 实际投递消息到目标节点
func (network *Network) deliverMessage(msg Message) {
	network.nodesMux.RLock()
	targetNode, exists := network.nodes[msg.NextHop]
	network.nodesMux.RUnlock()

	if !exists {
		log.Printf("Network: Target node %d not found, dropping message", msg.NextHop)
		return
	}

	// 尝试投递到目标节点的inbox
	select {
	case targetNode.inbox <- msg:
		log.Printf("Network: Delivered message type %d from %d to %d",
			msg.Type, msg.Src, msg.NextHop)
	default:
		// inbox满了，丢弃消息
		log.Printf("Network: Node %d inbox full, dropping message", msg.NextHop)
		network.statsMux.Lock()
		network.droppedMessages++
		network.statsMux.Unlock()
	}
}

// shouldDropPacket 根据丢包率决定是否丢包
func (network *Network) shouldDropPacket() bool {
	// 如果丢包率设置为0或更低，则从不丢包
	if network.packetLoss <= 0 {
		return false
	}
	// 如果丢包率设置为1或更高，则总是丢包
	if network.packetLoss >= 1.0 {
		return true
	}
	shouldDropPacket := rand.Float32() < network.packetLoss
	return shouldDropPacket
}

// -----------------------Public Methods-----------------------

// NewNetwork 创建 Network
func NewNetwork() *Network {
	return &Network{
		nodes:      make(map[uint32]*Node),
		latency:    10 * time.Millisecond, // 默认10ms延迟
		packetLoss: 0.0,                   // 默认无丢包
	}
}

// RegisterNode 注册节点到网络
func (network *Network) RegisterNode(node *Node) {
	network.nodesMux.Lock()
	defer network.nodesMux.Unlock()

	network.nodes[node.ID] = node
	log.Printf("Network: Registered node %d", node.ID)
}

// UnregisterNode 从网络注销节点
func (network *Network) UnregisterNode(nodeID uint32) {
	network.nodesMux.Lock()
	defer network.nodesMux.Unlock()

	delete(network.nodes, nodeID)
	log.Printf("Network: Unregistered node %d", nodeID)
}

// Send 发送消息的核心实现
func (network *Network) Send(msg Message) {
	network.statsMux.Lock()
	network.totalMessages++
	network.statsMux.Unlock()

	// 模拟丢包
	if network.shouldDropPacket() {
		network.statsMux.Lock()
		network.droppedMessages++
		network.statsMux.Unlock()
		log.Printf("Network: Packet dropped from %d to %d", msg.Src, msg.NextHop)
		return
	}

	// 模拟网络延迟
	if network.latency > 0 {
		go func() {
			time.Sleep(network.latency)
			network.deliverMessage(msg)
		}()
	} else {
		network.deliverMessage(msg)
	}
}

// GetMsgInfo 获取网络统计信息
func (network *Network) GetMsgInfo() (totalMsgs, droppedMsgs uint64) {
	network.statsMux.RLock()
	defer network.statsMux.RUnlock()
	return network.totalMessages, network.droppedMessages
}

// GetAllNodes 获取所有注册的节点ID
func (network *Network) GetAllNodes() []uint32 {
	network.nodesMux.RLock()
	defer network.nodesMux.RUnlock()

	nodeIDs := make([]uint32, 0, len(network.nodes))
	for id := range network.nodes {
		nodeIDs = append(nodeIDs, id)
	}
	return nodeIDs
}
