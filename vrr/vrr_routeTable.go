package vrr

import (
	"container/list"
	"log"
	"sync"

	"github.com/emirpasic/gods/trees/redblacktree"
)

// RoutingTableManager 封装了单个节点的路由表状态和操作。
type RoutingTableManager struct {
	ownerNode *Node                         // 指向拥有此管理器的节点
	lock      sync.RWMutex                  // 使用读写锁以优化性能
	routes    map[uint32]*RoutingTableEntry // 键是 PathId，值是路由条目
}

// rt_node represents a node in the routing table (red-black tree node)
type RbNode struct {
	Node                       *redblacktree.Node // The node in the red-black tree
	Endpoint                   uint32             // Endpoint value
	RoutingTableListByEndpoint list.List          // Routing entries associated with this node,[]routing_table_entry
}

// Struct for use in VRR Routing Table
type RoutingTableEntry struct {
	Ea     uint32 //endpoint A
	Eb     uint32 //endpoint B
	Na     uint32 //next A
	Nb     uint32 //next B
	PathId uint32 //Path ID
}

// NewRoutingTableManager 是构造函数。
func NewRoutingTableManager(owner *Node) *RoutingTableManager {
	return &RoutingTableManager{
		ownerNode: owner,
		routes:    make(map[uint32]*RoutingTableEntry),
	}
}

// getNextHop  在路由条目列表中查找最佳下一跳。
func (rt *RoutingTableManager) getNextHop(closestEndpoint uint32) uint32 {
	var bestRoute *RoutingTableEntry
	var nextHop uint32
	// 1.根据closestEndpoint查找所有相关路由条目。并知道Path ID最大的路由条目是最佳路由。
	for _, route := range rt.routes {
		if route.Ea == closestEndpoint || route.Eb == closestEndpoint {
			if bestRoute == nil || route.PathId > bestRoute.PathId {
				bestRoute = route
			}
		}
	}

	if bestRoute == nil {
		return 0
	}
	// 2. 根据找到的最佳路由路由条目，判断Ea和Eb那个是closestEndpoint确定下一跳。
	switch closestEndpoint {
	case bestRoute.Ea:
		nextHop = bestRoute.Na
	case bestRoute.Eb:
		nextHop = bestRoute.Nb
	default:
		nextHop = 0
	}
	return nextHop

}

// getClosestEndpoint 查找最接近目标的端点endpoint
func (rt *RoutingTableManager) getClosestEndpoint(dest uint32) uint32 {
	var closestEndpoint uint32
	var minDistance uint32 = ^uint32(0) // 最大的 uint32 值

	endpoints := make(map[uint32]bool)

	for _, route := range rt.routes {
		if route.Ea != 0 {
			endpoints[route.Ea] = true
		}
		if route.Eb != 0 {
			endpoints[route.Eb] = true
		}
	}

	if len(endpoints) == 0 {
		return 0 // 没有可用的外部端点
	}

	for ep := range endpoints {
		distance := get_diff(dest, ep)
		if distance < minDistance {
			minDistance = distance
			closestEndpoint = ep
		}
	}

	if closestEndpoint == 0 {
		return 0 // 未找到最近的端点
	}

	return closestEndpoint
}

// getClosestEndpointExclude 查找最接近目标的端点，但排除指定的源端点
func (rt *RoutingTableManager) getClosestEndpointExclude(dest uint32, excludeSrc uint32) uint32 {
	var closestEndpoint uint32
	var minDistance uint32 = ^uint32(0) // 最大的 uint32 值

	endpoints := make(map[uint32]bool)

	for _, route := range rt.routes {
		if route.Ea != 0 && route.Ea != excludeSrc {
			endpoints[route.Ea] = true
		}
		if route.Eb != 0 && route.Eb != excludeSrc {
			endpoints[route.Eb] = true
		}
	}

	if len(endpoints) == 0 {
		return 0 // 没有可用的外部端点
	}

	for ep := range endpoints {
		distance := get_diff(dest, ep)
		if distance < minDistance {
			minDistance = distance
			closestEndpoint = ep
		}
	}

	if closestEndpoint == 0 {
		return 0 // 未找到最近的端点
	}

	return closestEndpoint
}

// getEntriesByEndpoint 查找并返回所有以指定ID为端点的路由条目。
func (rt *RoutingTableManager) getTearDownPathsByEndpoint(endpoint uint32) []*RoutingTableEntry {
	rt.lock.RLock()
	defer rt.lock.RUnlock()

	var foundPaths []*RoutingTableEntry
	for _, route := range rt.routes {
		if route.Ea == endpoint || route.Eb == endpoint {
			foundPaths = append(foundPaths, route)
		}
	}
	return foundPaths
}

// -------------------VRR 论文方法实现---------------------------------------------
/*
Add(rt, <ea , eb , na , nb , pid> )
	adds the entry to the routing table unless there is already an
	entry with the same pid, ea
*/
// AddRoute  向路由表添加一个路由条目。
// to do :the entry with the same pid, ea should not be added
func (rt *RoutingTableManager) AddRoute(ea, eb, na, nb, pathID uint32) bool {
	rt.lock.Lock()
	defer rt.lock.Unlock()

	// 检查 PathId 是否已存在
	if _, exists := rt.routes[pathID]; exists {
		log.Printf("Node %d: Route with pathID %d already exists", rt.ownerNode.ID, pathID)
		return false
	}

	rt.routes[pathID] = &RoutingTableEntry{
		Ea:     ea,
		Eb:     eb,
		Na:     na,
		Nb:     nb,
		PathId: pathID,
	}
	log.Printf("Node %d: Added route (pathID: %d, ea: %d, eb: %d)", rt.ownerNode.ID, pathID, ea, eb)
	return true
}

/*
Remove(rt, <pid, ea> )
	removes and returns the entry identified by pid, ea from the routing table
*/
// RemoveRoute 从路由表中移除一个路由条目。
// tip:根据pathID删除条目即可，因为pathID是唯一标识
func (rt *RoutingTableManager) RemoveRoute(pathID uint32, endpoint uint32) *RoutingTableEntry {
	rt.lock.Lock()
	defer rt.lock.Unlock()

	entry, found := rt.routes[pathID]
	if !found {
		return nil
	}

	delete(rt.routes, pathID)
	log.Printf("Node %d: Removed route (pathID: %d)", rt.ownerNode.ID, pathID)
	return entry
}

/*
NextHop (rt, dst)
    endpoint := closest id to dst from Endpoints(rt)
    if (endpoint == me)
        return null
    return next hop towards endpoint in rt
*/
// GetNext 获取到目标地址的下一跳。

func (rt *RoutingTableManager) GetNext(dest uint32) uint32 {
	rt.lock.RLock()
	defer rt.lock.RUnlock()
	me := rt.ownerNode.ID

	closestEndpoint := rt.getClosestEndpoint(dest)
	if closestEndpoint == 0 || closestEndpoint == me {
		return 0
	}
	nextHop := rt.getNextHop(closestEndpoint)

	return nextHop
}

/*
NextHopExclude (rt, dst, src)
    endpoint := closest id to dst from Endpoints(rt) that is not src
    if (endpoint == me)
        return null
    return next hop towards endpoint in rt
*/
// GetNextExclude 获取到目标地址的下一跳，排除源地址。
func (rt *RoutingTableManager) GetNextExclude(dest uint32, src uint32) uint32 {
	rt.lock.RLock()
	defer rt.lock.RUnlock()
	me := rt.ownerNode.ID

	// 查找最接近的rbNode（排除源地址）
	closestEndpoint := rt.getClosestEndpointExclude(dest, src)
	if closestEndpoint == 0 || closestEndpoint == me {
		return 0 // 没有找到合适的端点
	}
	nextHop := rt.getNextHop(closestEndpoint)
	return nextHop
}

/*
TearDownPath(<pid,ea>, sender)
    < ea , eb , na , nb , pid := Remove(rt, <pid, ea>)
    for each (n ∈ {na , nb , sender})
        if (n != null ∧ n ∈ pset)
            vset’ := (sender != null) ? vset : null
            Send <teardown, <pid, ea> , vset’> to n
*/
// TearDownPath 撤销路径
func (rt *RoutingTableManager) TearDownPath(pathID, endpoint, sender uint32) {
	// 移除路由
	route := rt.RemoveRoute(endpoint, pathID)
	if route == nil {
		return
	}

	n := rt.ownerNode
	// 关键：根据sender决定是否包含vset
	var srcVsetToSend []uint32
	if sender == 0 {
		// sender为0，是正常路径维护（如vset协商失败），需要携带vset以同步状态
		srcVsetToSend = n.vsetManager.GetAll() // 包含vset
	} else {
		// sender不为0，是故障清理（如邻居丢失），不携带vset以触发对端重连
		srcVsetToSend = nil // 不包含vset
	}

	// 发送teardown消息
	if route.Na != 0 && n.psetManager.IsActiveLinkedPset(route.Na) {
		n.SendTeardown(pathID, endpoint, srcVsetToSend, route.Na)
	}
	if route.Nb != 0 && n.psetManager.IsActiveLinkedPset(route.Nb) {
		n.SendTeardown(pathID, endpoint, srcVsetToSend, route.Nb)
	}
}

// TearDownPathTo 拆除所有以指定ID为端点的vset-paths。
func (rt *RoutingTableManager) TearDownPathTo(endpoint uint32) {

	log.Printf("Node %d: Tearing down all paths to endpoint %d", rt.ownerNode.ID, endpoint)

	// 从路由表中查找所有相关的路径
	// 这个操作需要路由表管理器提供一个新方法来获取这些路径
	pathsToTearDown := rt.getTearDownPathsByEndpoint(endpoint)

	// 对找到的每一条路径，都发起一次TearDownPath
	// sender设置为0，因为这是由本节点的内部策略（vset bump）触发的，没有外部发送者。
	// 这表示一次“正常的路径维护”，而不是“网络故障”。
	for _, path := range pathsToTearDown {
		log.Printf("Node %d:   - Tearing down path (pid=%d, ea=%d)", rt.ownerNode.ID, path.PathId, path.Ea)
		// 使用 path.Ea 作为TearDownPath的端点参数，因为pid+ea是唯一标识
		rt.TearDownPath(path.PathId, path.Ea, 0)
	}
}
