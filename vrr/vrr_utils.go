package vrr

import (
	"log"
	"math/rand"
	"sync"
	"time"
)

// 全局随机数生成器（线程安全）
var (
	rng      = rand.New(rand.NewSource(time.Now().UnixNano()))
	rngMutex sync.Mutex
)

// generate random id,return []byte
func GenerateRandomBytes(size int) []byte {
	randomBytes := make([]byte, size)
	_, err := rand.Read(randomBytes)
	if err != nil {
		log.Printf("vrr: generateRandomBytes failed: %v", err)
		return nil
	}
	return randomBytes
}

// get_diff 计算两个无符号整数之间的绝对差值。
func get_diff(x, y uint32) uint32 {
	i := x - y //无符号值，不会出现负值
	j := y - x
	if i < j {
		return i
	}
	return j
}

// VrrNewPathID 作为 Node 的方法生成一个随机的 32 位路径 ID
// 确保生成的 ID 不与当前节点的 vset 中的任何节点 ID 冲突
func (n *Node) VrrNewPathID() uint32 {
	rngMutex.Lock()
	defer rngMutex.Unlock()

	// 获取当前节点的所有 vset 节点 ID
	vsetNodes := n.VsetManager.GetAll()

	// 将 vset 节点 ID 存入 map 用于快速查找
	existingIDs := make(map[uint32]bool)
	for _, nodeID := range vsetNodes {
		existingIDs[nodeID] = true
	}

	// 也要避免与自己的 ID 冲突
	existingIDs[n.ID] = true

	// 生成随机 ID，直到找到一个不冲突的
	var pathID uint32
	maxRetries := 100 // 防止无限循环

	for i := 0; i < maxRetries; i++ {
		pathID = rng.Uint32()

		// 避免使用特殊值
		if pathID == 0 || pathID == 0xFFFFFFFF {
			continue
		}

		// 检查是否与现有 ID 冲突
		if !existingIDs[pathID] {
			break
		}
	}

	log.Printf("Node %d: Generated new path ID: %d (0x%x)", n.ID, pathID, pathID)
	return pathID
}
