package vrr

import (
	"container/list"
	"log"
	"math/rand"
	"sync"
	"sync/atomic"
)

// Physical Set Setup
type PsetNode struct {
	NodeId    uint32
	Status    uint32
	Active    bool
	FailCount int32 //atomic
}

// PSetManager 封装了单个节点的物理邻居集状态和操作逻辑。
type PSetManager struct {
	ownerNode *Node        // 指向拥有此管理器的节点
	lock      sync.RWMutex // 使用读写锁以优化性能
	psetList  list.List    // 每个管理器实例都有自己的psetList
}

// NewPSetManager 是 PSetManager 的构造函数。
func NewPSetManager(owner *Node) *PSetManager {
	return &PSetManager{
		ownerNode: owner,
		// psetList 字段已经是 list.List 类型，它被零值初始化为一个可用的空列表。
	}
}

// Add  向物理邻居集中添加一个节点。
func (pms *PSetManager) Add(nodeID uint32, status uint32, Active bool) bool {
	pms.lock.Lock()
	defer pms.lock.Unlock()

	// 检查节点是否已存在
	for e := pms.psetList.Front(); e != nil; e = e.Next() {
		if e.Value.(*PsetNode).NodeId == nodeID {
			return false // 节点已存在
		}
	}

	// 创建新节点
	newNode := &PsetNode{
		NodeId: nodeID,
		Status: status,
		Active: Active,
	}
	atomic.StoreInt32(&newNode.FailCount, 0)

	// 添加到列表
	pms.psetList.PushBack(newNode)
	log.Printf("Node %d: PSet added neighbor %d", pms.ownerNode.ID, nodeID)
	return true
}

// Update 更新物理邻居集中一个节点的状态。
func (pms *PSetManager) Update(nodeID uint32, status uint32, Active bool) bool {
	pms.lock.Lock()
	defer pms.lock.Unlock()

	for e := pms.psetList.Front(); e != nil; e = e.Next() {
		pNode := e.Value.(*PsetNode)
		if pNode.NodeId == nodeID {
			pNode.Status = status
			pNode.Active = Active
			log.Printf("Node %d: PSet updated neighbor %d", pms.ownerNode.ID, nodeID)
			return true
		}
	}
	return false
}

// Find 在物理邻居集中查找一个节点。
func (pms *PSetManager) Find(nodeID uint32) *PsetNode {
	pms.lock.RLock()
	defer pms.lock.RUnlock()

	for e := pms.psetList.Front(); e != nil; e = e.Next() {
		if e.Value.(*PsetNode).NodeId == nodeID {
			return e.Value.(*PsetNode)
		}
	}
	return nil
}

// Contains 检查物理邻居集中是否存在指定的节点。
func (pms *PSetManager) Contains(nodeID uint32) bool {
	pms.lock.RLock() // 使用读锁
	defer pms.lock.RUnlock()

	for e := pms.psetList.Front(); e != nil; e = e.Next() {
		tmp := e.Value.(*PsetNode)
		if tmp.NodeId == nodeID {
			return true // 找到节点，返回 true
		}
	}

	return false // 遍历完成未找到，返回 false
}

// GetActive 获取物理邻居集中一个节点的活跃状态。
func (pms *PSetManager) GetActive(nodeID uint32) (bool, bool) {
	pms.lock.RLock() // 使用读锁，因为这是只读操作
	defer pms.lock.RUnlock()

	// 遍历列表查找节点
	for e := pms.psetList.Front(); e != nil; e = e.Next() {
		tmp := e.Value.(*PsetNode)
		if tmp.NodeId == nodeID {
			return tmp.Active, true // 返回活跃状态和 true 表示找到
		}
	}

	// 未找到节点，返回一个默认值 (0) 和 false
	return false, false
}

// GetStatus  获取物理邻居集中一个节点的状态。
func (pms *PSetManager) GetStatus(nodeID uint32) (uint32, bool) {
	pms.lock.RLock() // 使用读锁，因为这是只读操作
	defer pms.lock.RUnlock()

	for e := pms.psetList.Front(); e != nil; e = e.Next() {
		tmp := e.Value.(*PsetNode)
		if tmp.NodeId == nodeID {
			return tmp.Status, true // 返回状态和表示“找到”的布尔值
		}
	}
	// 如果未找到，返回一个默认值和表示“未找到”的布尔值
	// PSET_UNKNOWN 应该在您的常量定义中
	return PSET_UNKNOWN, false
}

// IncFailCount 原子地增加指定节点的失败计数。
// 注意：这个方法接收一个 *PsetNode 指针，因为它假设你已经通过 Find 找到了节点。
// 这样做可以避免在已经持有节点引用的情况下再次加锁和遍历。
func (pms *PSetManager) IncFailCount(nodeID uint32) (int32, bool) {
	// 这里我们复用 Find 方法
	pNode := pms.Find(nodeID)
	if pNode == nil {
		return -1, false
	}
	newValue := atomic.AddInt32(&pNode.FailCount, 1)
	log.Printf("Node %d: PSet incremented fail count for neighbor %d to %d", pms.ownerNode.ID, nodeID, newValue)
	return newValue, true
}

// ResetFailCount 原子地重置指定节点的失败计数。
func (pms *PSetManager) ResetFailCount(nodeID uint32) bool {
	// 这里我们复用 Find 方法
	pNode := pms.Find(nodeID)
	if pNode != nil {
		atomic.StoreInt32(&pNode.FailCount, 0)
		log.Printf("Node %d: PSet reset fail count for neighbor %d", pms.ownerNode.ID, nodeID)
		return true
	}
	return false
}

// ---------------------public api---------------------------------'
// Remove 从物理邻居集中移除一个节点。
func (pms *PSetManager) Remove(nodeID uint32) bool {
	pms.lock.Lock()
	defer pms.lock.Unlock()

	for e := pms.psetList.Front(); e != nil; e = e.Next() {
		if e.Value.(*PsetNode).NodeId == nodeID {
			pms.psetList.Remove(e)
			log.Printf("Node %d: PSet removed neighbor %d", pms.ownerNode.ID, nodeID)
			return true
		}
	}
	return false
}

// IsActiveLinkedPset 判断指定节点ID是否为当前节点的活跃且已链接的物理邻居
// 返回值：true表示是活跃且已链接的pset，false表示不是
func (pms *PSetManager) IsActiveLinkedPset(nodeID uint32) bool {
	pms.lock.RLock() // 使用读锁，因为这是只读操作
	defer pms.lock.RUnlock()

	// 遍历物理邻居列表
	for e := pms.psetList.Front(); e != nil; e = e.Next() {
		pNode := e.Value.(*PsetNode)
		if pNode.NodeId == nodeID {
			// 检查是否同时满足：已链接 AND 活跃
			return pNode.Status == PSET_LINKED && pNode.Active == true
		}
	}

	// 未找到该节点，返回 false
	return false
}

// -------------------VRR 论文方法实现------------------------------
/*
PickRandomActive(pset)
	returns a random physical neighbor that is Active
*/
// GetProxy 从活跃的物理邻居中随机选择一个作为代理。
func (pms *PSetManager) GetProxy() (uint32, bool) {
	pms.lock.RLock() // 使用读锁
	defer pms.lock.RUnlock()

	// 使用切片代替固定大小的数组，更灵活
	activeNodes := make([]uint32, 0, pms.psetList.Len())

	// 遍历列表，收集所有符合条件的节点
	for e := pms.psetList.Front(); e != nil; e = e.Next() {
		tmp := e.Value.(*PsetNode)
		if tmp.Status == PSET_LINKED && tmp.Active == true {
			activeNodes = append(activeNodes, tmp.NodeId)
		}
	}

	// 如果没有找到符合条件的节点，返回失败
	if len(activeNodes) == 0 {
		return 0, false
	}

	// 从符合条件的节点中随机选择一个
	// rand.Seed() 应该在程序启动时调用一次，而不是每次都调用
	r := rand.Intn(len(activeNodes))
	proxy := activeNodes[r]

	return proxy, true
}
