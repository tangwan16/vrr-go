package network

import (
	"log"
	"math/rand"
	"sync"
	"time"

	"github.com/tangwan16/vrr-go/vrr"
)

// Network 模拟链路层 fabric（进程内交换/转发器）
type Network struct {
	Nodes    map[uint32]*vrr.Node // 所有节点的映射表
	nodesMux sync.RWMutex         // 保护节点映射表的读写锁

	// 网络延迟和丢包模拟参数
	Latency    time.Duration // 模拟网络延迟
	PacketLoss float32       // 丢包率 (0.0 - 1.0)

	// 统计信息
	TotalMessages   uint64       // 总发送消息数
	DroppedMessages uint64       // 丢失消息数
	statsMux        sync.RWMutex // 统计信息锁
}

// deliverMessage 实际投递消息到目标节点
func (network *Network) deliverMessage(msg vrr.Message) {
	network.nodesMux.RLock()
	targetNode, exists := network.Nodes[msg.NextHop]
	network.nodesMux.RUnlock()

	if !exists {
		log.Printf("Network: Target node %d not found, dropping message", msg.NextHop)
		return
	}

	// 尝试投递到目标节点的inbox
	select {
	case targetNode.InboxChan <- msg:
		log.Printf("network: Delivered message type %d from %d to %d",
			msg.Type, msg.Src, msg.NextHop)
	default:
		// inbox满了，丢弃消息
		log.Printf("network: Node %d inbox full, dropping message", msg.NextHop)
		network.statsMux.Lock()
		network.DroppedMessages++
		network.statsMux.Unlock()
	}
}

// shouldDropPacket 根据丢包率决定是否丢包
func (network *Network) shouldDropPacket() bool {
	// 如果丢包率设置为0或更低，则从不丢包
	if network.PacketLoss <= 0 {
		return false
	}
	// 如果丢包率设置为1或更高，则总是丢包
	if network.PacketLoss >= 1.0 {
		return true
	}
	shouldDropPacket := rand.Float32() < network.PacketLoss
	return shouldDropPacket
}

// -----------------------Public Methods-----------------------

// NewNetwork 创建 Network
func NewNetwork(Latency time.Duration, PacketLoss float32) *Network {
	return &Network{
		Nodes:      make(map[uint32]*vrr.Node),
		Latency:    Latency,
		PacketLoss: PacketLoss,
	}
}

// RegisterNode 注册节点到网络
func (network *Network) RegisterNode(node *vrr.Node) {
	network.nodesMux.Lock()
	defer network.nodesMux.Unlock()

	network.Nodes[node.ID] = node
	log.Printf("network: Registered node %d", node.ID)
}

// UnregisterNode 从网络注销节点
func (network *Network) UnregisterNode(nodeID uint32) {
	network.nodesMux.Lock()
	defer network.nodesMux.Unlock()

	delete(network.Nodes, nodeID)
	log.Printf("Network: Unregistered node %d", nodeID)
}

// Send 发送消息的核心实现
func (network *Network) Send(msg vrr.Message) {
	network.statsMux.Lock()
	network.TotalMessages++
	network.statsMux.Unlock()

	// 模拟丢包
	if network.shouldDropPacket() {
		network.statsMux.Lock()
		network.DroppedMessages++
		network.statsMux.Unlock()
		log.Printf("network: Packet dropped from %d to %d", msg.Src, msg.NextHop)
		return
	}

	// 模拟网络延迟
	if network.Latency > 0 {
		go func() {
			time.Sleep(network.Latency)
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
	return network.TotalMessages, network.DroppedMessages
}

// GetAllNodes 获取所有注册的节点ID
func (network *Network) GetAllNodes() []uint32 {
	network.nodesMux.RLock()
	defer network.nodesMux.RUnlock()

	nodeIDs := make([]uint32, 0, len(network.Nodes))
	for id := range network.Nodes {
		nodeIDs = append(nodeIDs, id)
	}
	return nodeIDs
}
