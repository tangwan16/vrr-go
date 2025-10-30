package vrr

import (
	"log"
	"math/rand"
	"sync/atomic"
	"time"
	// "github.com/tangwan16/vrr-go/Network"
)

// Start 启动节点的消息处理循环，与广播周期性HELLO消息
func (n *Node) Start() {
	n.wg.Add(2) //启动两个goroutine

	// 启动一个 goroutine 来处理传入的消息，sendSetupReq,Setup
	go func() {
		defer n.wg.Done() // 确保此 goroutine 退出时，计数器减一
		for {
			select {
			case msg := <-n.InboxChan:
				n.rcvMessage(msg)
			case <-n.StopChan:
				return
			}
		}
	}()

	// 启动一个goroutine单独处理周期性消息如Hello
	go func() {
		defer n.wg.Done() // 确保此 goroutine 退出时，计数器减一
		// 定义 HELLO 发送周期
		/* 		helloTicker := time.NewTicker(300 * time.Millisecond) // 每0.3秒向HelloTicker对象内部通道C发送时间信号tick
		   		defer helloTicker.Stop() */
		// --- 使用带有 Jitter 的 Timer 替代固定的 Ticker ---
		const baseInterval = 500 * time.Millisecond
		const jitter = 300 * time.Millisecond // 随机抖动范围
		// 计算一个随机的下一次触发时间
		randomDuration := baseInterval + time.Duration(rand.Int63n(int64(jitter)*2)-int64(jitter))
		timer := time.NewTimer(randomDuration)
		defer timer.Stop()

		for {
			select {
			case <-timer.C:
				n.DetectFailures()
				n.ActiveTimeout()
				n.SendHello()
				// 重置计时器以进行下一次触发
				randomDuration = baseInterval + time.Duration(rand.Int63n(int64(jitter)*2)-int64(jitter))
				timer.Reset(randomDuration)
			case <-n.StopChan:
				return
			}
		}
	}()

	log.Printf("Node %d: Started message processing and periodic HELLO sender", n.ID)
}

// Stop 停止节点
func (n *Node) Stop() {
	n.stopOnce.Do(func() {
		// 1. 发送停止信号
		close(n.StopChan)
		// 2. 等待所有 goroutine 真正退出
		n.wg.Wait()
		// 3. 在所有任务都结束后，打印统一的日志
		log.Printf("Node %d: Closed message processing and periodic HELLO sender", n.ID)
	})
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
	// fmt.Printf("psetManager、VsetManager、psetStateManager、routingTable created for node %d done\n", n.ID)

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

// detectFailures 检测失败的邻居节点
// to do:为什么上来直接增加失败计数？
func (n *Node) DetectFailures() {
	for e := n.PsetManager.psetList.Front(); e != nil; e = e.Next() {
		pNode := e.Value.(*PsetNode)

		// 定期增加失败计数，只有收到消息，才会重置失败计数
		count, _ := n.IncFailCount(pNode.NodeId)

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

// activeTimeout 处理活跃状态超时（每个时间单位调用一次）
func (n *Node) ActiveTimeout() {
	// 如果已经活跃，直接返回
	if n.Active {
		return
	}

	// 超时计数器递增
	n.Timeout++
	// log.Printf("Node %d: Timeout: (%d)", n.ID, n.Timeout)

	// 达到超时阈值时激活节点
	if n.Timeout >= VRR_ACTIVE_TIMEOUT {
		n.Active = true
		log.Printf("Node %d: Activated after Timeout (%d ticks)", n.ID, n.Timeout)
		// 自己自举成功后，这会抢占其他可能即将超时的节点，并引导它们加入自己的网络。
		n.SendHello()

	}
}

// ResetActiveTimeout 重置节点活跃超时
func (n *Node) ResetActiveTimeout() {
	n.lock.Lock()
	defer n.lock.Unlock()
	n.Timeout = 0
}

// ResetFailCount 原子地重置指定节点的失败计数。
func (n *Node) ResetFailCount(nodeID uint32) bool {
	pNode := n.PsetManager.find(nodeID)
	if pNode != nil {
		atomic.StoreInt32(&pNode.FailCount, 0)
		// log.Printf("Node %d: Pset reset fail count for neighbor Node %d", n.ID, nodeID)
		return true
	}
	return false
}

// IncFailCount 原子地增加指定节点的失败计数。
func (n *Node) IncFailCount(nodeID uint32) (int32, bool) {
	pNode := n.PsetManager.find(nodeID)
	if pNode == nil {
		return -1, false
	}
	newValue := atomic.AddInt32(&pNode.FailCount, 1)
	// log.Printf("Node %d: PSet incremented fail count for neighbor %d to %d", n.ID, nodeID, newValue)
	return newValue, true
}
