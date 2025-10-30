package main

import (
	"log"
	"testing"
	"time"

	"github.com/tangwan16/vrr-go/network"
	"github.com/tangwan16/vrr-go/vrr"
)

func TestSendData(t *testing.T) {
	log.Println("--- Running Test: SendData ---")
	network := network.NewNetwork(50*time.Millisecond, 0.0)

	// --- 定义拓扑 ---
	// Subnet 1: Node 2, Node 5
	// Subnet 2: Node 2, Node 3, Node 4
	// Subnet 3: Node 6
	// Router: Node 2 (连接 Subnet 1 和 Subnet 2)

	node5 := vrr.NewNode(8085, network)
	node2 := vrr.NewNode(8082, network)
	node3 := vrr.NewNode(8083, network)
	node4 := vrr.NewNode(8084, network)
	node6 := vrr.NewNode(8086, network)

	// --- 注册节点到子网 ---
	network.RegisterNode(node5, 1) // node5 在子网1并且active=true开始构建虚拟网络
	node5.SetActive(true)
	network.RegisterNode(node2, 1, 2) // node2 是路由器, 在子网1和2
	network.RegisterNode(node3, 2)    // node3 在子网2
	network.RegisterNode(node4, 2)    // node4 在子网2,active=false
	network.RegisterNode(node6, 3)    // node5 在孤立的子网3,active=false

	nodes := []*vrr.Node{node6, node2, node3, node4, node5}

	// 启动所有节点
	for _, n := range nodes {
		n.Start()
		defer n.Stop()
	}

	// 等待足够长的时间让节点们自己完成握手，建立虚拟网络和vset-paths
	log.Println("\n--- Waiting for virtual network and vset-paths to complete... ---")
	time.Sleep(3 * time.Second)
	printAllRoutes(nodes)

	log.Println("\n--- : Simulating 'Send Data' from Node 5 to Node 3 ---")
	payload := []byte("Hello from Node 5 to Node 3!")
	node5.SendData(node3.ID, payload)
	time.Sleep(2 * time.Second) // 等待一段时间以确保数据包传输完成
	totalMsgs, droppedMsgs := network.GetMsgInfo()
	log.Printf("Simulation completed: Total messages: %d, Dropped: %d", totalMsgs, droppedMsgs)

}

/* 2025/10/30 20:12:28 --- Current Routing Tables ---
2025/10/30 20:12:28 Node 8086 -> RoutingTable: {empty}
2025/10/30 20:12:28 Node 8082 -> RoutingTable:
        - PathID: 334087316, Ea: 8085, Eb: 8082, Na: 8085, Nb: 0
        - PathID: 722807410, Ea: 8085, Eb: 8084, Na: 8085, Nb: 8084
        - PathID: 1658627646, Ea: 8085, Eb: 8083, Na: 8085, Nb: 8083
        - PathID: 3790633383, Ea: 8082, Eb: 8083, Na: 0, Nb: 8083
        - PathID: 4182614828, Ea: 8082, Eb: 8084, Na: 0, Nb: 8084
2025/10/30 20:12:28 Node 8083 -> RoutingTable:
        - PathID: 1539543547, Ea: 8083, Eb: 8084, Na: 0, Nb: 8084
        - PathID: 1658627646, Ea: 8085, Eb: 8083, Na: 8082, Nb: 0
        - PathID: 2056844202, Ea: 8084, Eb: 8083, Na: 8084, Nb: 0
        - PathID: 3790633383, Ea: 8082, Eb: 8083, Na: 8082, Nb: 0
2025/10/30 20:12:28 Node 8084 -> RoutingTable:
        - PathID: 722807410, Ea: 8085, Eb: 8084, Na: 8082, Nb: 0
        - PathID: 1539543547, Ea: 8083, Eb: 8084, Na: 8083, Nb: 0
        - PathID: 2056844202, Ea: 8084, Eb: 8083, Na: 0, Nb: 8083
        - PathID: 4182614828, Ea: 8082, Eb: 8084, Na: 8082, Nb: 0
2025/10/30 20:12:28 Node 8085 -> RoutingTable:
        - PathID: 334087316, Ea: 8085, Eb: 8082, Na: 0, Nb: 8082
        - PathID: 722807410, Ea: 8085, Eb: 8084, Na: 0, Nb: 8082
        - PathID: 1658627646, Ea: 8085, Eb: 8083, Na: 0, Nb: 8082
2025/10/30 20:12:28 ------------------------------
2025/10/30 20:12:28
--- : Simulating 'Send Data' from Node 5 to Node 3 ---
2025/10/30 20:12:28 Node 8085: SendData to dest=8083 via nextHop=8082
2025/10/30 20:12:28 Node 8082: Received message type VRR_DATA from Node 8085 to Node 8083
2025/10/30 20:12:28 Node 8082: Forwarded data to 8083 via 8083
2025/10/30 20:12:28 Node 8082: Received periodic HELLO Msg from Node 8085 in same subnet
2025/10/30 20:12:28 Node 8083: Received message type VRR_DATA from Node 8085 to Node 8083
2025/10/30 20:12:28 Node 8083: Data packet delivered from 8085, payload size: 28 */
