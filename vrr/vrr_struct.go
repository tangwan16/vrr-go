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

// Payload 接口定义了所有具体消息内容的共同行为。
// isPayload 是一个标记方法，用于类型约束，确保只有我们定义的消息体才能被赋值。
type Payload interface {
	isPayload()
}

// Message 现在是消息的“信封”，包含路由和元数据。
type Message struct {
	Type    uint8  // 消息类型，用于解析 Payload
	Src     uint32 // 消息的逻辑发起者ID
	Dst     uint32 // 消息的最终逻辑目的地ID, 广播为0
	NextHop uint32 // 下一跳节点ID（用于转发）
	Sender  uint32 // 实际发送者节点ID（上一跳）

	Payload Payload // 消息的具体内容
}

func (*HelloPayload) isPayload()     {}
func (*SetupReqPayload) isPayload()  {}
func (*SetupPayload) isPayload()     {}
func (*SetupFailPayload) isPayload() {}
func (*TeardownPayload) isPayload()  {}
func (*DataPayload) isPayload()      {}

// HelloPayload 对应 HELLO 消息
type HelloPayload struct {
	SenderActive           bool
	HelloInfoLinkActive    []uint32
	HelloInfoLinkNotActive []uint32
	HelloInfoPending       []uint32
}

// SetupReqPayload 对应 SETUP_REQ 消息
type SetupReqPayload struct {
	Proxy uint32
	Vset_ []uint32
}

type SetupPayload struct {
	Pid   uint32
	Proxy uint32
	Vset_ []uint32
}

type SetupFailPayload struct {
	Proxy uint32
	Vset_ []uint32
}

type TeardownPayload struct {
	Pid      uint32
	Endpoint uint32
	Vset_    []uint32
}

type DataPayload struct {
	Data []byte
}

// Node 模拟一个 VRR 节点
type Node struct {
	ID        uint32       // 节点的唯一标识符
	InboxChan chan Message // 消息接收通道，模拟网络接口的接收队列

	Network Networker // 对模拟网络的引用，用于发送消息

	lock   sync.RWMutex // 保护节点内部状态（如active）的读写锁
	Active bool         // 节点是否在虚拟集合和路由中 receive setup会设置为active=true

	Timeout int // 活跃状态超时计数器，对应 vrr_node.Timeout

	// --- 状态管理器 ---
	PsetManager      *PsetManager         // 物理邻居集管理器
	VsetManager      *VsetManager         // 虚拟邻居集管理器
	RoutingTable     *RoutingTableManager // 路由表管理器
	PsetStateManager *PsetStateManager    // 物理邻居集管理器

	// 并发控制
	StopChan chan struct{} // 用于通知goroutine停止的信号通道
	stopOnce sync.Once     // 确保 StopChan 只关闭一次
	wg       sync.WaitGroup
}
