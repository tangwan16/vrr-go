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

	SubnetTopology map[uint32][]uint32 // 新增：子网拓扑。key: 子网ID, value: 该子网中的节点ID列表
	NodeToSubnet   map[uint32][]uint32 // 新增：节点到子网的反向映射。key: 节点ID, value: 该节点所属的子网ID列表
	topologyMux    sync.RWMutex
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
		/* 		log.Printf("Network: Delivered message type %s from Node %d to Node %d",
		vrr.GetMessageTypeString(msg.Type), msg.Src, msg.NextHop) */
	default:
		// inbox满了，丢弃消息
		log.Printf("Network: Node %d inbox full, dropping message", msg.NextHop)
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
		Nodes:          make(map[uint32]*vrr.Node),
		Latency:        Latency,
		PacketLoss:     PacketLoss,
		SubnetTopology: make(map[uint32][]uint32), // 初始化
		NodeToSubnet:   make(map[uint32][]uint32), // 初始化
	}
}

// RegisterNode 注册节点到子网
func (network *Network) RegisterNode(node *vrr.Node, subnetIDs ...uint32) {
	network.nodesMux.Lock()
	defer network.nodesMux.Unlock()
	network.Nodes[node.ID] = node

	network.topologyMux.Lock()
	defer network.topologyMux.Unlock()
	network.NodeToSubnet[node.ID] = subnetIDs
	for _, subnetID := range subnetIDs {
		network.SubnetTopology[subnetID] = append(network.SubnetTopology[subnetID], node.ID)
	}

	log.Printf("Network: Registered node %d to subnet(s) %v", node.ID, subnetIDs)
}

// UnregisterNode 从网络注销节点 (完全移除)
func (network *Network) UnregisterNode(nodeID uint32) {
	// 严格遵守加锁顺序: 1. nodesMux, 2. topologyMux
	network.nodesMux.Lock()
	defer network.nodesMux.Unlock()
	network.topologyMux.Lock()
	defer network.topologyMux.Unlock()

	// 1. 从拓扑信息中移除节点
	subnets, exists := network.NodeToSubnet[nodeID]
	if exists {
		for _, subnetID := range subnets {
			nodesInSubnet := network.SubnetTopology[subnetID]
			newNodesInSubnet := make([]uint32, 0, len(nodesInSubnet)-1)
			for _, id := range nodesInSubnet {
				if id != nodeID {
					newNodesInSubnet = append(newNodesInSubnet, id)
				}
			}
			network.SubnetTopology[subnetID] = newNodesInSubnet
		}
		delete(network.NodeToSubnet, nodeID)
	}

	// 2. 从主节点列表中移除节点
	delete(network.Nodes, nodeID)

	log.Printf("Network: Unregistered node %d", nodeID)
}

// UnregisterNodeFromSubnets 将节点从指定的子网列表中注销
func (network *Network) UnregisterNodeFromSubnets(nodeID uint32, subnetsToLeave ...uint32) {
	if len(subnetsToLeave) == 0 {
		return
	}

	// 严格遵守加锁顺序: 1. nodesMux, 2. topologyMux
	network.nodesMux.Lock()
	defer network.nodesMux.Unlock()
	network.topologyMux.Lock()
	defer network.topologyMux.Unlock()

	// 为了快速查找，将要离开的子网放入一个map
	leaveMap := make(map[uint32]bool, len(subnetsToLeave))
	for _, s := range subnetsToLeave {
		leaveMap[s] = true
	}

	// 1. 更新 SubnetTopology
	for subnetID := range leaveMap {
		if nodesInSubnet, ok := network.SubnetTopology[subnetID]; ok {
			newNodesInSubnet := make([]uint32, 0, len(nodesInSubnet)-1)
			for _, id := range nodesInSubnet {
				if id != nodeID {
					newNodesInSubnet = append(newNodesInSubnet, id)
				}
			}
			network.SubnetTopology[subnetID] = newNodesInSubnet
		}
	}

	// 2. 更新 NodeToSubnet
	if currentSubnets, ok := network.NodeToSubnet[nodeID]; ok {
		newCurrentSubnets := make([]uint32, 0, len(currentSubnets))
		for _, subnetID := range currentSubnets {
			if !leaveMap[subnetID] {
				newCurrentSubnets = append(newCurrentSubnets, subnetID)
			}
		}

		// 关键修复：如果节点不再属于任何子网，则从两个映射中都删除它
		if len(newCurrentSubnets) == 0 {
			delete(network.NodeToSubnet, nodeID)
			delete(network.Nodes, nodeID) // <--- 你指出的问题在这里被修复
		} else {
			network.NodeToSubnet[nodeID] = newCurrentSubnets
		}
	}

	log.Printf("Network: Unregistered node %d from subnet(s) %v", nodeID, subnetsToLeave)
}

// Send 发送消息的核心实现
func (network *Network) Send(msg vrr.Message) {
	// --- 广播逻辑 ---
	if msg.NextHop == 0 {
		network.topologyMux.RLock()
		defer network.topologyMux.RUnlock()

		// 找到发送者所在的子网
		senderSubnets, ok := network.NodeToSubnet[msg.Src]
		if !ok || len(senderSubnets) == 0 {
			log.Printf("Network: Broadcast failed. Sender %d is not in any subnet.", msg.Src)
			return
		}

		// 使用一个 map 来防止向同一个节点发送多次广播（当一个节点属于多个子网时）
		sentTo := make(map[uint32]bool)

		// 遍历发送者所在的所有子网
		for _, subnetID := range senderSubnets {
			// log.Printf("Network: Broadcasting message type %s from %d in subnet %d", vrr.GetMessageTypeString(msg.Type), msg.Src, subnetID)
			// 向该子网内的所有节点广播
			for _, targetNodeID := range network.SubnetTopology[subnetID] {
				// 节点不向自己广播，且不重复广播
				if targetNodeID == msg.Src || sentTo[targetNodeID] {
					continue
				}

				broadcastMsg := msg
				broadcastMsg.NextHop = targetNodeID
				network.sendMessage(broadcastMsg)
				sentTo[targetNodeID] = true
			}
		}
		return
	}

	// --- 单播逻辑 (保持不变) ---
	network.sendMessage(msg)
}

// sendMessage 封装了单个消息的发送逻辑（延迟和丢包）
// 这是从旧的 Send 方法中提取出来的
func (network *Network) sendMessage(msg vrr.Message) {
	network.statsMux.Lock()
	network.TotalMessages++
	network.statsMux.Unlock()

	// 模拟丢包
	if network.shouldDropPacket() {
		network.statsMux.Lock()
		network.DroppedMessages++
		network.statsMux.Unlock()
		log.Printf("Network: Packet dropped from Node %d to Node %d", msg.Src, msg.NextHop)
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
