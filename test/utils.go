package main

import (
	"log"

	"github.com/tangwan16/vrr-go/vrr"
)

// printAllPset 是一个辅助函数，用于打印所有节点的 PSet 状态
func printAllPset(nodes []*vrr.Node) {
	log.Println("--- Current PSet ---")
	for _, n := range nodes {
		// 调用我们刚刚在 PsetManager 中添加的 String() 方法
		log.Printf("Node %d -> %s", n.ID, n.PsetManager.String())
	}
	log.Println("---------------------------")
}

// printAllPsetState 是一个辅助函数，用于打印所有节点的 PSet 状态
func printAllPsetState(nodes []*vrr.Node) {
	log.Println("--- Current PSet States ---")
	for _, n := range nodes {
		// 修改这里：调用 n.PsetStateManager.String() 而不是 n.PsetManager.String()
		log.Printf("Node %d -> %s", n.ID, n.PsetStateManager.String())
	}
	log.Println("---------------------------")
}
