package vrr

import (
	"fmt"
	"log"

	"github.com/tangwan16/vrr-go/network"
)

// DetectFailures 检测失败的邻居节点
// to do:为什么上来直接增加失败计数？
func (n *Node) DetectFailures() {
	for e := n.psetManager.psetList.Front(); e != nil; e = e.Next() {
		pNode := e.Value.(*PsetNode)

		// 增加失败计数
		count, _ := n.psetManager.IncFailCount(pNode.NodeId)

		// 检查是否需要标记为失败
		if count >= VRR_FAIL_TIMEOUT && pNode.Status != PSET_FAILED {
			log.Printf("Node %d: Marking failed node: %d", n.ID, pNode.NodeId)
			n.psetManager.Update(pNode.NodeId, PSET_FAILED, pNode.Active)
			n.psetStateManager.Update()
		}

		// 检查是否需要删除节点
		if count >= 2*VRR_FAIL_TIMEOUT {
			log.Printf("Node %d: Deleting failed node: %d", n.ID, pNode.NodeId)
			n.psetManager.Remove(pNode.NodeId)
			// n.psetStateManager.Update(n.psetManager)
		}
	}
}

// ResetActiveTimeout 重置活跃超时（修正函数名）
func (n *Node) ResetActiveTimeout() {
	n.lock.Lock()
	defer n.lock.Unlock()
	n.timeout = 0
}

// ActiveTimeoutTick 处理活跃状态超时（每个时间单位调用一次）
func (n *Node) ActiveTimeoutTick() {
	// 如果已经活跃，直接返回
	if n.active {
		return
	}

	// 超时计数器递增
	n.timeout++

	// 达到超时阈值时激活节点
	if n.timeout >= VRR_ACTIVE_TIMEOUT {
		n.active = true
		log.Printf("Node %d: Activated after timeout (%d ticks)", n.ID, n.timeout)
	}
}

// Start 启动节点的消息处理循环
func (n *Node) Start() {
	go func() {
		for {
			select {
			case msg := <-n.inbox:
				n.rcvMessage(msg)
			case <-n.stop:
				log.Printf("Node %d: Stopping message processing", n.ID)
				return
			}
		}
	}()

	log.Printf("Node %d: Started message processing", n.ID)
}

// Stop 停止节点
func (n *Node) Stop() {
	close(n.stop)
	log.Printf("Node %d: Stop signal sent", n.ID)
}

// NewNode 创建节点
func NewNode(id uint32, network *network.Network) *Node {
	n := &Node{
		ID:      id,
		inbox:   make(chan Message, 256),
		stop:    make(chan struct{}),
		network: network,
		active:  false,
	}

	// 为这个新节点创建一套独立的管理器
	n.psetManager = NewPSetManager(n)
	fmt.Println("psetManager created for node done")
	n.vsetManager = NewVSetManager(n)
	fmt.Println("vsetManager created for node done")
	n.routingTable = NewRoutingTableManager(n)
	fmt.Println("routingTable created for node done")
	n.psetStateManager = NewPsetStateManager(n)
	fmt.Println("psetStateManager created for node done")

	return n
}
