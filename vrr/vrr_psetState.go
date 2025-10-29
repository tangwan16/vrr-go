package vrr

import (
	"fmt"
	"log"
	"strings"
	"sync"
)

const (
	VRR_PSET_SIZE = 20

	// 邻居状态转移类型
	TRANS_LINKED  = 0
	TRANS_PENDING = 1
	TRANS_MISSING = 2

	// 邻居状态
	PSET_LINKED  = 0
	PSET_PENDING = 1
	PSET_FAILED  = 2
	PSET_UNKNOWN = 3
)

var (
	psetStates = []string{"linked", "pending", "failed", "unknown"}
	psetTrans  = []string{"linked", "pending", "missing"}
	// 状态转移表
	// 维度: [当前状态][收到的邻居关系] -> 下一状态
	helloTrans = [4][3]uint32{
		//              TRANS_LINKED, TRANS_PENDING, TRANS_MISSING
		/* PSET_LINKED */ {PSET_LINKED, PSET_LINKED, PSET_FAILED},
		/* PSET_PENDING */ {PSET_LINKED, PSET_LINKED, PSET_PENDING},
		/* PSET_FAILED */ {PSET_FAILED, PSET_PENDING, PSET_PENDING},
		/* PSET_UNKNOWN */ {PSET_FAILED, PSET_LINKED, PSET_PENDING},
	}
)

type PsetStateManager struct {
	ownerNode           *Node        // 指向拥有此管理器的节点
	lock                sync.RWMutex // 使用读写锁以优化性能
	LinkActive          []uint32     // 活跃的已链接节点
	LinkNotActive       []uint32     // 非活跃的已链接节点
	Pending             []uint32     // 待定状态节点
	psetStateUpdateChan chan PsetStateUpdate
}

// --- 邻居更新处理 ---

// PsetStateUpdate 结构用于传递 HELLO 报文解析出的更新信息
type PsetStateUpdate struct {
	node   uint32
	trans  int
	active bool
}

// NewPsetStateManager 创建新的 PsetStateManager
func NewPsetStateManager(owner *Node) *PsetStateManager {
	psm := &PsetStateManager{
		ownerNode:           owner,
		LinkActive:          make([]uint32, 0, VRR_PSET_SIZE),
		LinkNotActive:       make([]uint32, 0, VRR_PSET_SIZE),
		Pending:             make([]uint32, 0, VRR_PSET_SIZE),
		psetStateUpdateChan: make(chan PsetStateUpdate, 100),
	}
	// 启动后台工作者goroutine
	go psm.updateHandler()
	return psm
}

// ScheduleUpdate 对应C代码中的 schedule_work，将更新任务放入队列
func (psm *PsetStateManager) ScheduleUpdate(update PsetStateUpdate) {
	// 非阻塞发送，如果队列满了，可以打印日志或丢弃，防止阻塞网络处理goroutine
	select {
	case psm.psetStateUpdateChan <- update:
		// 任务成功入队
	default:
		log.Printf("Node %d: Pset update queue is full. Discarding update for node %d.", psm.ownerNode.ID, update.node)
	}
}

// updateHandler 是在后台运行的工作者，对应C代码的 pset_update_handler
func (psm *PsetStateManager) updateHandler() {
	n := psm.ownerNode
	me := n.ID
	// log.Printf("Node %d: started to receive Hello Msg for updating Pset state", me.ID)

	// TODO：是否要加锁以安全地访问和修改PSetManager的状态
	// 使用 for-range 循环不断地从channel中接收任务
	for tmp := range psm.psetStateUpdateChan {
		curState := n.PsetManager.GetStatus(tmp.node)
		nextState := helloTrans[curState][tmp.trans]
		curActive, _ := n.PsetManager.GetActive(tmp.node)

		log.Printf("Node %d: Pset update for Node %d: %s[%s] ==> %s",
			me, tmp.node, psetStates[curState], psetTrans[tmp.trans], psetStates[nextState])

		if curState == PSET_UNKNOWN {
			// 发送Hello消息节点为新节点，添加到PSet中
			n.PsetManager.Add(tmp.node, nextState, tmp.active)
			psm.Update()
		} else if curState != nextState || curActive != tmp.active {
			// 状态或活跃性有变化，更新PSet
			n.PsetManager.Update(tmp.node, nextState, tmp.active)
			psm.Update()
		}

		// 如果当前节点自己是非活跃节点(未在虚拟邻居集中),找到一个已加入网络活跃的节点，发送setup_req请求
		if !n.Active && tmp.active && nextState == PSET_LINKED {
			log.Printf("Node %d: New Active/linked neighbor %d found. Sending setup_req to self via proxy %d.", me, tmp.node, tmp.node)
			vset := n.VsetManager.GetAll()
			n.SendSetupReq(me, me, me, tmp.node, tmp.node, vset)
		}
	}
	log.Printf("Node %d: ended to receive Hello Msg for updating Pset state", me)
}

// Update ：根据pset 更新 PsetState
func (psm *PsetStateManager) Update() {
	// 清空当前状态
	pm := psm.ownerNode.PsetManager
	psm.LinkActive = psm.LinkActive[:0]
	psm.LinkNotActive = psm.LinkNotActive[:0]
	psm.Pending = psm.Pending[:0]

	for e := pm.psetList.Front(); e != nil; e = e.Next() {
		pNode := e.Value.(*PsetNode)
		switch pNode.Status {
		case PSET_LINKED:
			if pNode.Active {
				psm.LinkActive = append(psm.LinkActive, pNode.NodeId)
			} else {
				psm.LinkNotActive = append(psm.LinkNotActive, pNode.NodeId)
			}
		case PSET_PENDING:
			psm.Pending = append(psm.Pending, pNode.NodeId)
		}
	}
}

// -------------------public api-----------------------------
// String 返回 PsetStateManager 状态的可读字符串表示形式
func (psm *PsetStateManager) String() string {
	psm.lock.RLock()
	defer psm.lock.RUnlock()

	// 使用 strings.Builder 来高效构建字符串
	var builder strings.Builder
	builder.WriteString("PSetState: {")
	builder.WriteString(fmt.Sprintf("LinkedActive: %v, ", psm.LinkActive))
	builder.WriteString(fmt.Sprintf("LinkedNotActive: %v, ", psm.LinkNotActive))
	builder.WriteString(fmt.Sprintf("Pending: %v", psm.Pending))
	builder.WriteString("}")

	return builder.String()
}
