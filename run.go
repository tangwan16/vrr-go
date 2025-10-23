package main

import (
	"log"
	"time"

	vrr "./vrr"
)

func main() {
	// 创建全局网络
	network := vrr.NewNetwork()

	// 设置网络参数
	network.latency = 50 * time.Millisecond // 50ms网络延迟
	network.packetLoss = 0.0                // 0%丢包率

	// 创建节点
	node1 := vrrNewNode(1, network)
	node2 := NewNode(2, network)
	node3 := NewNode(3, network)
	node4 := NewNode(4, network)

	// 注册节点到网络
	network.RegisterNode(node1)
	network.RegisterNode(node2)
	network.RegisterNode(node3)
	network.RegisterNode(node4)

	// 启动节点
	node1.Start()
	node2.Start()
	node3.Start()
	node4.Start()

	// 模拟节点间建立物理邻居关系
	setupPhysicalNeighbors(node1, node2)
	setupPhysicalNeighbors(node2, node3)
	setupPhysicalNeighbors(node3, node4)

	// 运行模拟
	runSimulation(node1, node2, node3, node4)

	// 清理
	time.Sleep(1 * time.Second)
	node1.Stop()
	node2.Stop()
	node3.Stop()

	// 输出统计信息
	totalMsgs, droppedMsgs := network.GetMsgInfo()
	log.Printf("Simulation completed: Total messages: %d, Dropped: %d", totalMsgs, droppedMsgs)
}

// 建立物理邻居关系
func setupPhysicalNeighbors(node1 *Node, node2 *Node) {
	// 节点1和节点2互为物理邻居
	node1.psetManager.Add(node2.ID, PSET_LINKED, true)
	node2.psetManager.Add(node1.ID, PSET_LINKED, true)

	log.Printf("Physical neighbor relationships established between %d and %d", node1.ID, node2.ID)
}

// 运行模拟场景
func runSimulation(node1, node2, node3, node4 *Node) {
	log.Println("Starting VRR simulation...")

	// 场景1：节点1发送HELLO广播
	time.Sleep(100 * time.Millisecond)
	node1.SendHelloPkt()

	// 场景2：节点1向节点3发起路径建立
	time.Sleep(200 * time.Millisecond)
	// node1.SendSetupReq(node1.ID, node3.ID, node2.ID)

	// 场景3：发送数据
	time.Sleep(500 * time.Millisecond)
	payload := []byte("Hello from Node 1 to Node 3!")
	node1.SendData(node3.ID, payload)

	// 让消息处理完成
	time.Sleep(1 * time.Second)

	log.Println("Simulation scenarios completed")
}
