package vrr

import (
	"sync"
)

const (
	VRR_ID_LEN = 4

	VRR_VSET_SIZE = 4

	VRR_FAIL_TIMEOUT = 4 /* multiple of delay to mark
	 * failed nodes */
	VRR_ACTIVE_TIMEOUT = 8 /* multiple of delay to activate
	* this node without virtual
	* neighbors */
)

type Networker interface {
	Send(msg Message) // 假设 Message 类型也在 vrr 包或其依赖的包中
}

// Message 是进程内“链路层”消息结构，替代 skb + eth + vrr header
type Message struct {
	Type uint8
	Src  uint32 // 消息的逻辑发起者ID。
	Dst  uint32 // 消息的最终逻辑目的地ID,广播消息，设置为0

	Pid      uint32 //路径ID。用于 SETUP, TEARDOWN 等消息。
	Endpoint uint32 //端点值。用于 TEARDOWN 消息。
	Proxy    uint32 //代理节点ID。用于 SETUP_REQ, SETUP 等消息。

	Vset_   []uint32 // 虚拟邻居集合.用于 SETUP_REQ, SETUP, SETUP_FAIL, TEARDOWN 消息。
	NextHop uint32   // 下一跳节点ID（用于转发）
	Sender  uint32   // 实际发送者节点ID（上一跳）

	Payload []byte // 应用层数据。用于 DATA 类型的消息。

	// hello 消息
	SenderActive           bool
	HelloInfoLinkActive    []uint32
	HelloInfoLinkNotActive []uint32
	HelloInfoPending       []uint32
}

// Node 模拟一个 VRR 节点
type Node struct {
	ID uint32 // 节点的唯一标识符

	StopChan chan struct{} // 用于通知goroutine停止的信号通道

	Network Networker // 对模拟网络的引用，用于发送消息

	lock   sync.RWMutex // 保护节点内部状态（如active）的读写锁
	Active bool         // 节点是否活跃，对应 vrr_node.Active

	Timeout int // 活跃状态超时计数器，对应 vrr_node.Timeout

	// --- 状态管理器 ---
	// 每个节点都拥有自己独立的状态管理器实例
	// --- 核心组件 ---
	InboxChan        chan Message         // 消息接收通道，模拟网络接口的接收队列
	PsetManager      *PSetManager         // 物理邻居集管理器
	VsetManager      *VSetManager         // 虚拟邻居集管理器
	RoutingTable     *RoutingTableManager // 路由表管理器
	PsetStateManager *PsetStateManager    // 物理邻居集管理器
}
