package main

import (
	"log"
	"testing" // 导入 testing 包
	"time"

	"github.com/tangwan16/vrr-go/network"
	"github.com/tangwan16/vrr-go/vrr"
)

// 测试节点间的 HELLO 握手过程
func TestHelloHandshake(t *testing.T) {
	log.Println("--- Running Test: HelloHandshake ---")
	network := network.NewNetwork(50*time.Millisecond, 0.0)

	// --- 定义拓扑 ---
	// Subnet 1: Node 5, Node 2
	// Subnet 2: Node 2, Node 3, Node 4
	// Router: Node 2 (连接 Subnet 1 和 Subnet 2)
	// 孤立节点: Node 6 (在 Subnet 3)

	node5 := vrr.NewNode(8085, network)
	node2 := vrr.NewNode(8082, network)
	node3 := vrr.NewNode(8083, network)
	node4 := vrr.NewNode(8084, network)
	node6 := vrr.NewNode(8086, network)

	// --- 注册节点到子网 ---
	network.RegisterNode(node5, 1) // node5 在子网1并且active=true开始构建虚拟网络
	node5.SetActive(true)
	network.RegisterNode(node2, 1, 2) // node2 是路由器, 在子网1和2
	// node2.SetActive(true)
	network.RegisterNode(node3, 2) // node3 在子网2
	// node3.SetActive(true)
	network.RegisterNode(node4, 2) // node4 在子网2,active=false
	// node4.SetActive(false)
	network.RegisterNode(node6, 3) // node5 在孤立的子网3,active=false
	// node5.SetActive(false)

	nodes := []*vrr.Node{node6, node2, node3, node4, node5}

	// 启动所有节点
	for _, n := range nodes {
		n.Start()
		defer n.Stop()
	}

	// 初始状态
	/* 	log.Println("\n--- Initial State ---")
	   	printAllPsetState(nodes) */

	// 等待足够长的时间让节点们自己完成握手
	// 至少需要 2-3 个 HELLO 周期才能稳定到 LINKED 状态
	log.Println("\n--- Waiting for autonomous HELLO handshake to complete... ---")
	time.Sleep(3 * time.Second)

	// 检查最终状态
	log.Println("\n--- Final State after autonomous handshake ---")
	printAllPsetState(nodes)

	log.Println("--- Test HelloHandshake Finished ---")
	printAllVsets(nodes)
	printAllRoutes(nodes)

	totalMsgs, droppedMsgs := network.GetMsgInfo()
	log.Printf("Simulation completed: Total messages: %d, Dropped: %d", totalMsgs, droppedMsgs)
}

// 运行测试
// 样例
/* --- Final State after autonomous handshake ---
2025/10/29 21:24:35 --- Current PSet States ---
2025/10/29 21:24:35 Node 8086 -> PSetState: {LinkedActive: [], LinkedNotActive: [], Pending: []}
2025/10/29 21:24:35 Node 8082 -> PSetState: {LinkedActive: [8084 8085 8083], LinkedNotActive: [], Pending: []}
2025/10/29 21:24:35 Node 8083 -> PSetState: {LinkedActive: [8082 8084], LinkedNotActive: [], Pending: []}
2025/10/29 21:24:35 Node 8084 -> PSetState: {LinkedActive: [8083 8082], LinkedNotActive: [], Pending: []}
2025/10/29 21:24:35 Node 8085 -> PSetState: {LinkedActive: [8082], LinkedNotActive: [], Pending: []}
2025/10/29 21:24:35 ---------------------------
2025/10/29 21:24:35 --- Test HelloHandshake Finished ---
2025/10/29 21:24:35 --- Current VSet ---
2025/10/29 21:24:35 Node 8086 -> VSet: []
2025/10/29 21:24:35 Node 8082 -> VSet: [8083 8084 8085]
2025/10/29 21:24:35 Node 8083 -> VSet: [8082 8084 8085]
2025/10/29 21:24:35 Node 8084 -> VSet: [8082 8083 8085]
2025/10/29 21:24:35 Node 8085 -> VSet: [8082 8083 8084]
2025/10/29 21:24:35 --------------------
2025/10/29 21:24:35 --- Current Routing Tables ---
2025/10/29 21:24:35 Node 8086 -> RoutingTable: {empty}
2025/10/29 21:24:35 Node 8082 -> RoutingTable:
        - PathID: 420270169, Ea: 8084, Eb: 8085, Na: 8083, Nb: 8085
        - PathID: 515004063, Ea: 8085, Eb: 8082, Na: 8085, Nb: 0
        - PathID: 649850686, Ea: 8082, Eb: 8083, Na: 0, Nb: 8083
        - PathID: 1298432611, Ea: 8082, Eb: 8084, Na: 0, Nb: 8084
        - PathID: 1587357221, Ea: 8085, Eb: 8083, Na: 8085, Nb: 8083
2025/10/29 21:24:35 Node 8083 -> RoutingTable:
        - PathID: 420270169, Ea: 8084, Eb: 8085, Na: 8084, Nb: 8082
        - PathID: 649850686, Ea: 8082, Eb: 8083, Na: 8082, Nb: 0
        - PathID: 1587357221, Ea: 8085, Eb: 8083, Na: 8082, Nb: 0
        - PathID: 3231993192, Ea: 8083, Eb: 8084, Na: 0, Nb: 8084
2025/10/29 21:24:35 Node 8084 -> RoutingTable:
        - PathID: 420270169, Ea: 8084, Eb: 8085, Na: 0, Nb: 8083
        - PathID: 1298432611, Ea: 8082, Eb: 8084, Na: 8082, Nb: 0
        - PathID: 3231993192, Ea: 8083, Eb: 8084, Na: 8083, Nb: 0
2025/10/29 21:24:35 Node 8085 -> RoutingTable:
        - PathID: 420270169, Ea: 8084, Eb: 8085, Na: 8082, Nb: 0
        - PathID: 515004063, Ea: 8085, Eb: 8082, Na: 0, Nb: 8082
        - PathID: 1587357221, Ea: 8085, Eb: 8083, Na: 0, Nb: 8082
2025/10/29 21:24:35 ------------------------------
2025/10/29 21:24:35 Simulation completed: Total messages: 99, Dropped: 0*/
