package vrr

import (
	"container/list"
	"fmt"
	"log"
	"math"
	"sort"
	"sync"
)

// Virtual Set Setup
type VsetNode struct {
	NodeId    uint32
	DiffLeft  int //ME 向左多少距离能到 vset_node
	DiffRight int //ME 向右多少距离能到 vset_node
}

// VsetManager 封装了单个节点的虚拟邻居集状态和操作逻辑。
type VsetManager struct {
	ownerNode *Node        // 指向拥有此管理器的节点
	lock      sync.RWMutex // 使用读写锁以优化性能
	vsetList  list.List    // 每个管理器实例都有自己的vsetList
}

// NewVSetManager 是 VsetManager 的构造函数。
func NewVsetManager(owner *Node) *VsetManager {
	return &VsetManager{
		ownerNode: owner,
		// vsetList 字段已经是 list.List 类型，它被零值初始化为一个可用的空列表。
	}
}

// insertNode 将一个新节点插入到虚拟邻居集中。
// 这是一个内部方法，应在持有写锁的情况下调用。
func (vm *VsetManager) insertNode(nodeId uint32) {
	// 使用 vm.ownerNode.ID 替代全局的 ME
	meID := vm.ownerNode.ID

	// 创建一个新的 vset_list 条目
	tmp := &VsetNode{
		NodeId: nodeId,
		// 根据节点与所有者的关系计算 diff
		DiffLeft:  int(math.MaxUint32 - get_diff(nodeId, meID)),
		DiffRight: int(get_diff(nodeId, meID)),
	}

	if nodeId < meID {
		tmp.DiffLeft = int(get_diff(nodeId, meID))
		tmp.DiffRight = int(math.MaxUint32 - get_diff(nodeId, meID))
	}

	// 将新条目添加到此管理器的 vsetList 中
	vm.vsetList.PushBack(tmp)

	// log.Printf("Node %d: VSet inserted neighbor %d", meID, tmp.NodeId)
}

// bump 检查VSet大小，如果超出限制则“挤出”一个节点。
// 这是一个内部方法，应在持有写锁的情况下调用。
func (vm *VsetManager) bump() (uint32, bool) {
	radius := VRR_VSET_SIZE / 2
	vsetSize := vm.vsetList.Len()

	// 如果VSet大小未超限，则无需操作
	if vsetSize <= VRR_VSET_SIZE {
		return 0, false
	}

	left := make([]int, vsetSize)
	right := make([]int, vsetSize)
	i := 0

	// 填充左右差异数组
	for e := vm.vsetList.Front(); e != nil; e = e.Next() {
		tmp := e.Value.(*VsetNode)
		left[i] = tmp.DiffLeft
		right[i] = tmp.DiffRight
		i++
	}

	// 排序
	sort.Ints(left)
	sort.Ints(right)

	// 找到并移除被“挤出”的节点
	for e := vm.vsetList.Front(); e != nil; e = e.Next() {
		tmp := e.Value.(*VsetNode)
		if tmp.DiffLeft == left[radius] && tmp.DiffRight == right[radius] {
			removeNodeID := tmp.NodeId
			vm.vsetList.Remove(e) // 从列表中移除
			log.Printf("Node %d: VSet bumped neighbor %d", vm.ownerNode.ID, removeNodeID)
			return removeNodeID, true
		}
	}

	log.Printf("Node %d: VSet bump algorithm failed!", vm.ownerNode.ID)
	return 0, false
}

// Add 向虚拟邻居集中添加一个节点。
func (vm *VsetManager) Add(node uint32) (uint32, bool) {
	vm.lock.Lock() // 获取写锁
	defer vm.lock.Unlock()

	// 检查节点是否已存在
	for e := vm.vsetList.Front(); e != nil; e = e.Next() {
		if e.Value.(*VsetNode).NodeId == node {
			return 0, false // 节点已存在
		}
	}

	// 插入新节点
	vm.insertNode(node)

	// 检查是否需要“挤出”节点
	return vm.bump()
}

// GetAll 获取VSet中所有节点的ID。
func (vm *VsetManager) GetAll() []uint32 {
	vm.lock.RLock() // 获取读锁
	defer vm.lock.RUnlock()

	vsetAll := make([]uint32, 0, vm.vsetList.Len())
	for e := vm.vsetList.Front(); e != nil; e = e.Next() {
		vsetAll = append(vsetAll, e.Value.(*VsetNode).NodeId)
	}

	return vsetAll
}

// -------------------VRR 论文方法实现
/*
ShouldAdd(vset, id)
	sorts the identifiers in vset union {id, me} and returns true if id
	should be in the vset;
*/
// ShouldAdd  检查一个新节点是否应该被添加到VSet中。
// to do: 直接给定参数vset,不要使用list遍历
func (vm *VsetManager) ShouldAdd(node uint32) bool {
	vm.lock.RLock() // 获取读锁
	defer vm.lock.RUnlock()

	meID := vm.ownerNode.ID

	// 获取新节点的diffLeft和diffRight
	diffLeft := int(get_diff(node, meID))
	diffRight := int(get_diff(node, meID))
	if node > meID {
		diffLeft = math.MaxUint32 - diffLeft
	}
	if node < meID {
		diffRight = math.MaxUint32 - diffRight
	}

	// 检查节点是否已存在
	for e := vm.vsetList.Front(); e != nil; e = e.Next() {
		if e.Value.(*VsetNode).NodeId == node {
			return false
		}
	}

	vsetSize := vm.vsetList.Len()
	// 如果VSet未满，直接添加
	if vsetSize < VRR_VSET_SIZE {
		return true // VSet未满，可以直接添加
	}

	radius := VRR_VSET_SIZE / 2
	left := make([]int, vsetSize)
	right := make([]int, vsetSize)
	i := 0

	for e := vm.vsetList.Front(); e != nil; e = e.Next() {
		tmp := e.Value.(*VsetNode)
		left[i] = tmp.DiffLeft
		right[i] = tmp.DiffRight
		i++
	}

	sort.Ints(left)
	sort.Ints(right)

	// 检查新节点是否比现有节点“更近”
	for i = 0; i < radius; i++ {
		if left[i] > diffLeft || right[i] > diffRight {
			return true
		}
	}

	return false
}

/*
Remove(vset, id)
	removes node id from the vset
*/
// Remove 从虚拟邻居集中移除一个节点。
func (vm *VsetManager) Remove(node uint32) bool {
	vm.lock.Lock() // 获取写锁
	defer vm.lock.Unlock()

	for e := vm.vsetList.Front(); e != nil; e = e.Next() {
		if e.Value.(*VsetNode).NodeId == node {
			vm.vsetList.Remove(e)
			log.Printf("Node %d: VSet removed neighbor %d", vm.ownerNode.ID, node)
			return true // 成功移除
		}
	}

	return false // 未找到节点
}

/*
Add (vset, src, vset')
   for each (id ∈ vset’)
       if (ShouldAdd(vset, id))
           proxy := PickRandomActive(pset)
           Send setup req, me, id, proxy, vset to proxy
   if (src != null ∧ ShouldAdd(vset,src))
       add src to vset and any nodes removed to rem
       for each (id ∈ rem) TearDownPathTo(id)
       return true;
   return false;
*/

// Add 处理一个新的路径请求，可能会向 vset 中添加节点或发送 setup_req
func (n *Node) Add(vset []uint32, src uint32, vset_ []uint32) bool {
	me := n.ID
	// log.Printf("Node %d: VrrAdd from src=%d with vset=%v", me, src, vset_)

	// 对 vset_ 中的每个节点，检查是否应该添加，如果应该添加，则选择一个代理并发送 setup_req
	for _, id := range vset_ {
		if n.VsetManager.ShouldAdd(id) {
			proxy, ok := n.PsetManager.GetProxy()
			if ok {
				n.SendSetupReq(me, id, me, proxy, proxy, vset)
			}
			// log.Printf("Node %d: Sending setup_req to dest=%d via proxy=%d", n.ID, id, proxy)
		}
	}
	// AddMsgSrcToLocalVset(src,vset_)
	// 如果 src 不为零且应该添加到 vset 中
	if src != 0 && n.VsetManager.ShouldAdd(src) {
		log.Printf("Node %d: Added src Node %d to vset", n.ID, src)
		removedNodeId, _ := n.VsetManager.Add(src)
		if removedNodeId != 0 {
			n.RoutingTable.TearDownPathTo(removedNodeId)
			log.Printf("Node %d: Should tear down path to removed node %d", n.ID, removedNodeId)
		}

		return true
	}
	// AddMsgSrcToLocalVset(0,vset_)
	// 否则返回 false
	return false
}

// String 返回 VsetManager 状态的可读字符串表示形式
func (vm *VsetManager) String() string {
	vm.lock.RLock()
	defer vm.lock.RUnlock()

	if vm.vsetList.Len() == 0 {
		return "VSet: []"
	}

	// 提取所有 NodeId
	ids := make([]uint32, 0, vm.vsetList.Len())
	for e := vm.vsetList.Front(); e != nil; e = e.Next() {
		ids = append(ids, e.Value.(*VsetNode).NodeId)
	}

	// 为了保证输出顺序一致，对 ID 进行排序
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })

	// 将排序后的 ID 列表格式化成字符串
	return fmt.Sprintf("VSet: %v", ids)
}
