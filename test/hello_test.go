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
	// Subnet 2: Node 3, Node 4
	// Router: Node 2 (连接 Subnet 1 和 Subnet 2)
	// 孤立节点: Node 5 (在 Subnet 3)

	node1 := vrr.NewNode(1, network)
	node2 := vrr.NewNode(2, network)
	node3 := vrr.NewNode(3, network)
	node4 := vrr.NewNode(4, network)
	node5 := vrr.NewNode(5, network)

	// --- 注册节点到子网 ---
	network.RegisterNode(node1, 1)    // node1 在子网1
	network.RegisterNode(node2, 1, 2) // node2 是路由器, 在子网1和2
	network.RegisterNode(node3, 2)    // node3 在子网2
	network.RegisterNode(node4, 2)    // node4 在子网2
	network.RegisterNode(node5, 3)    // node5 在孤立的子网3
	nodes := []*vrr.Node{node1, node2, node3, node4, node5}

	// 启动所有节点
	for _, n := range nodes {
		n.Start()
		defer n.Stop()
	}

	// 初始状态
	log.Println("\n--- Initial State ---")
	printAllPsets(nodes)

	// --- Round 1 ---
	log.Println("\n--- Round 1 ---")
	log.Println("Node 1 sends HELLO...")
	node1.SendHello()
	time.Sleep(200 * time.Millisecond)
	// printAllPsets(nodes)

	log.Println("Node 3 sends HELLO...")
	node3.SendHello()
	time.Sleep(200 * time.Millisecond)
	printAllPsets(nodes)

	log.Println("Node 2 (Router) sends HELLO...")
	node2.SendHello()
	time.Sleep(200 * time.Millisecond)
	printAllPsets(nodes)

	node4.SendHello()
	time.Sleep(200 * time.Millisecond)
	printAllPsets(nodes)

	node5.SendHello()
	time.Sleep(200 * time.Millisecond)
	printAllPsets(nodes)

	// --- Round 2 ---
	log.Println("\n--- Round 2 (to establish LINKED state) ---")
	log.Println("Node 1 sends HELLO...")
	node1.SendHello()
	time.Sleep(200 * time.Millisecond)

	log.Println("Node 3 sends HELLO...")
	node3.SendHello()
	time.Sleep(200 * time.Millisecond)

	log.Println("Node 2 (Router) sends HELLO...")
	node2.SendHello()
	time.Sleep(200 * time.Millisecond)

	node4.SendHello()
	time.Sleep(200 * time.Millisecond)
	printAllPsets(nodes)

	node5.SendHello()
	time.Sleep(200 * time.Millisecond)
	printAllPsets(nodes)

	log.Println("\n--- Final State ---")
	printAllPsets(nodes)

	log.Println("--- Test HelloHandshake Finished ---")
}

// printAllPsets 是一个辅助函数，用于打印所有节点的 PSet 状态
func printAllPsets(nodes []*vrr.Node) {
	log.Println("--- Current PSet States ---")
	for _, n := range nodes {
		// 调用我们刚刚在 PsetManager 中添加的 String() 方法
		log.Printf("Node %d -> %s", n.ID, n.PsetManager.String())
	}
	log.Println("---------------------------")
}
