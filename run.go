package main

import (
	"log"
	"time"

	"github.com/tangwan16/vrr-go/network"
	"github.com/tangwan16/vrr-go/vrr"
)

func main() {
	// 创建全局网络,设置延迟50ms和丢包率为0%
	// 这模拟了一个单一的 Layer 2 广播域（像一个大交换机）
	network := network.NewNetwork(50*time.Millisecond, 0.0)

	// --- 阶段 1: 准备一个稳定的、已存在的网络 ---
	log.Println("--- Phase 1: Setting up an existing stable network (Node 1, 2) ---")
	node1 := vrr.NewNode(1, network)
	node2 := vrr.NewNode(2, network)

	// 1. 节点存在于网络世界中
	network.RegisterNode(node1)
	network.RegisterNode(node2)

	// 2. 定义物理拓扑（插上网线）
	setupPhysicalNeighbors(node1, node2)

	// 3. 启动节点（开始监听和处理消息）
	node1.Start()
	node2.Start()

	// 4. 手动激活 node1 和 node2，并让他们互相发送HELLO来建立LINKED状态
	// 这模拟了一个已经运行了一段时间的稳定网络
	node1.SendHello()
	time.Sleep(1000 * time.Millisecond) // 等待消息处理
	node2.SendHello()
	time.Sleep(1000 * time.Millisecond) // 等待消息处理
	// log.Printf("Node 1 and 2 are now active and mutually LINKED.")

	// --- 阶段 2: 模拟新节点 Node 3 加入 ---
	log.Println("\n--- Phase 2: A new node (Node 3) joins the network ---")
	node3 := vrr.NewNode(3, network)

	// 1. Node 3 存在于网络世界中
	network.RegisterNode(node3)

	// 2. 将 Node 3 的网线插入到与 Node 2 相连的交换机上
	setupPhysicalNeighbors(node2, node3)

	// 3. Node 3 上电，开始运行协议栈
	node3.Start()
	// log.Printf("Node 3 created, started, and physically linked to Node 2. It is currently NOT ACTIVE.")

	// --- 阶段 3: 观察协议的自驱动“节点加入”流程 ---
	// runNodeJoinSimulation(node2, node3)

	// --- 阶段 4: 清理 ---
	// log.Println("\n--- Phase 4: Cleaning up ---")
	// time.Sleep(2 * time.Second) // 等待所有模拟消息跑完

	node1.Stop()
	node2.Stop()
	node3.Stop()

	totalMsgs, droppedMsgs := network.GetMsgInfo()
	log.Printf("Simulation completed: Total messages: %d, Dropped: %d", totalMsgs, droppedMsgs)
}

// setupPhysicalNeighbors 模拟“插上网线”，定义物理拓扑
func setupPhysicalNeighbors(node1 *vrr.Node, node2 *vrr.Node) {
	// 在Pset中创建条目，状态为UNKNOWN，等待HELLO协议去协商
	node1.PsetManager.Add(node2.ID, vrr.PSET_UNKNOWN, false)
	node2.PsetManager.Add(node1.ID, vrr.PSET_UNKNOWN, false)
	log.Printf("Physical link defined between Node %d and Node %d.", node1.ID, node2.ID)
}

// runNodeJoinSimulation 运行并解说“节点加入”的完整流程
func runNodeJoinSimulation(proxyNode, newNode *vrr.Node) {
	log.Println("\n--- Phase 3: Simulating the self-driving 'Node Joins' protocol ---")

	// 步骤 1: 通过 Hello 消息寻找代理节点
	log.Printf("\n[Step 1] Proxy Node %d broadcasts a HELLO message.", proxyNode.ID)
	proxyNode.SendHello()
	// 新节点 newNode(3) 会收到这个HELLO。它的 receiveHello -> updateHandler 逻辑会处理这个消息，
	// 经过几个来回的HELLO交换（这里简化为一次触发），newNode会将proxyNode识别为一个活跃的LINKED邻居。
	time.Sleep(200 * time.Millisecond) // 等待HELLO处理和状态机更新

	// 步骤 2: 新节点自动发送 setup_req
	log.Printf("\n[Step 2] New Node %d automatically sends <setup_req, src=%d, dst=%d> via its proxy Node %d.",
		newNode.ID, newNode.ID, newNode.ID, proxyNode.ID)
	log.Println("         (This is triggered INTERNALLY by the protocol in vrr_psetState.go, not called from run.go)")
	// 此时，vrr_psetState.go 中的 `if !me.Active && tmp.Active && nextState == PSET_LINKED` 条件被满足，
	// 触发了 `SendSetupReq`。

	// 步骤 3 & 4: 消息被路由到最近的节点(y)，y回复setup
	log.Println("\n[Step 3 & 4] The request is routed to the closest v-neighbor (e.g., Node 1), which replies with a <setup> message containing its vset.")
	// 这个过程由网络和节点的receiveSetupReq/receiveSetup自动完成。
	time.Sleep(500 * time.Millisecond) // 等待 setup_req 的路由和 setup 的回复

	// 步骤 5 & 6: 新节点收到setup，处理vset，并向其他虚拟邻居发送setup_req
	log.Println("\n[Step 5 & 6] New Node %d receives the <setup> message, processes the vset, and may send further setup_reqs to establish all vset-paths.")
	time.Sleep(500 * time.Millisecond)

	// 步骤 7: 新节点成为活跃状态
	log.Println("\n[Step 7] After establishing paths, New Node %d becomes active.", newNode.ID)
	// 在真实协议中，当与所有vset邻居的路径都建立后，节点会自我激活。
	// 在我们的模拟中，我们可以在这里检查vset状态或简单地手动激活来表示流程结束。

	log.Printf("Node %d is now ACTIVE.", newNode.ID)

	// 最终测试：新节点现在应该能路由到远端的虚拟邻居
	log.Printf("\n--- Final Test: New Node %d sends data to distant Node 1 ---", newNode.ID)
	payload := []byte("Data from newly joined node!")
	if newNode.SendData(1, payload) {
		log.Println("Test successful: Node 3 found a route to Node 1 and sent data.")
	} else {
		log.Println("Test FAILED: Node 3 could not find a route to Node 1.")
	}
}
