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
	// Subnet 1: Node 1, Node 2
	// Subnet 2: Node 2, Node 3, Node 4
	// Router: Node 2 (连接 Subnet 1 和 Subnet 2)
	// 孤立节点: Node 5 (在 Subnet 3)

	node1 := vrr.NewNode(8081, network)
	node2 := vrr.NewNode(8082, network)
	node3 := vrr.NewNode(8083, network)
	node4 := vrr.NewNode(8084, network)
	node5 := vrr.NewNode(8085, network)

	// --- 注册节点到子网 ---
	network.RegisterNode(node1, 1) // node1 在子网1并且active=true开始构建虚拟网络
	node1.SetActive(true)
	network.RegisterNode(node2, 1, 2) // node2 是路由器, 在子网1和2
	// node2.SetActive(true)
	network.RegisterNode(node3, 2) // node3 在子网2
	// node3.SetActive(true)
	network.RegisterNode(node4, 2) // node4 在子网2,active=false
	// node4.SetActive(false)
	network.RegisterNode(node5, 3) // node5 在孤立的子网3,active=false
	// node5.SetActive(false)

	nodes := []*vrr.Node{node1, node2, node3, node4, node5}

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

	printAllRoutes(nodes)

	totalMsgs, droppedMsgs := network.GetMsgInfo()
	log.Printf("Simulation completed: Total messages: %d, Dropped: %d", totalMsgs, droppedMsgs)
}
