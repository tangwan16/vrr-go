package vrr

import (
	"fmt"
	"log"
	"time"
	// "github.com/tangwan16/vrr-go/Network"
)

// Start 启动节点的消息处理循环，与广播周期性HELLO消息
func (n *Node) Start() {
	// 启动一个 goroutine 来处理传入的消息，sendSetupReq,Setup
	go func() {
		for {
			select {
			case msg := <-n.InboxChan:
				n.rcvMessage(msg)
			case <-n.StopChan:
				log.Printf("Node %d: Stopping message processing", n.ID)
				return
			}
		}
	}()

	// 启动一个goroutine单独处理周期性消息如Hello
	go func() {
		// 定义 HELLO 发送周期
		helloTicker := time.NewTicker(300 * time.Millisecond) // 每3秒向HelloTicker对象内部通道C发送时间信号tick
		defer helloTicker.Stop()

		for {
			select {
			case <-helloTicker.C:
				n.SendHello()
			case <-n.StopChan:
				log.Printf("Node %d: Stopping periodic HELLO sender", n.ID)
				return
			}
		}
	}()

	log.Printf("Node %d: Started message processing", n.ID)
}

// Stop 停止节点
func (n *Node) Stop() {
	close(n.StopChan)
	log.Printf("Node %d: Stop signal sent", n.ID)
}

// NewNode 创建节点
func NewNode(id uint32, Network Networker) *Node {
	n := &Node{
		ID:        id,
		InboxChan: make(chan Message, 256),
		StopChan:  make(chan struct{}),
		Network:   Network,
		Active:    false,
	}

	// 为这个新节点创建一套独立的管理器
	n.PsetManager = NewPsetManager(n)
	n.VsetManager = NewVsetManager(n)
	n.RoutingTable = NewRoutingTableManager(n)
	n.PsetStateManager = NewPsetStateManager(n)
	fmt.Printf("psetManager、VsetManager、psetStateManager、routingTable created for node %d done\n", n.ID)

	return n
}

// --------------------public api-----------------------------
// SetActive 设置节点活跃状态
// to do :修改active的逻辑
func (n *Node) SetActive(active bool) {
	n.lock.Lock()
	defer n.lock.Unlock()
	n.Active = active
}

// DetectFailures 检测失败的邻居节点
// to do:为什么上来直接增加失败计数？
func (n *Node) DetectFailures() {
	for e := n.PsetManager.psetList.Front(); e != nil; e = e.Next() {
		pNode := e.Value.(*PsetNode)

		// 增加失败计数
		count, _ := n.PsetManager.IncFailCount(pNode.NodeId)

		// 检查是否需要标记为失败
		if count >= VRR_FAIL_TIMEOUT && pNode.Status != PSET_FAILED {
			log.Printf("Node %d: Marking failed node: %d", n.ID, pNode.NodeId)
			n.PsetManager.Update(pNode.NodeId, PSET_FAILED, pNode.Active)
			n.PsetStateManager.Update()
		}

		// 检查是否需要删除节点
		if count >= 2*VRR_FAIL_TIMEOUT {
			log.Printf("Node %d: Deleting failed node: %d", n.ID, pNode.NodeId)
			n.PsetManager.Remove(pNode.NodeId)
			// n.psetStateManager.Update(n.psetManager)
		}
	}
}

// ResetActiveTimeout 重置活跃超时（修正函数名）
func (n *Node) ResetActiveTimeout() {
	n.lock.Lock()
	defer n.lock.Unlock()
	n.Timeout = 0
}

// ActiveTimeoutTick 处理活跃状态超时（每个时间单位调用一次）
func (n *Node) ActiveTimeoutTick() {
	// 如果已经活跃，直接返回
	if n.Active {
		return
	}

	// 超时计数器递增
	n.Timeout++

	// 达到超时阈值时激活节点
	if n.Timeout >= VRR_ACTIVE_TIMEOUT {
		n.Active = true
		log.Printf("Node %d: Activated after Timeout (%d ticks)", n.ID, n.Timeout)
	}
}
