package vrr

import (
	"log"
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

type PsetStateManager struct {
	ownerNode           *Node        // 指向拥有此管理器的节点
	lock                sync.RWMutex // 使用读写锁以优化性能
	LinkActive          []uint32     // 活跃的已链接节点
	LinkNotActive       []uint32     // 非活跃的已链接节点
	Pending             []uint32     // 待定状态节点
	psetStateUpdateChan chan PsetStateUpdate
}

var psetStates = []string{"linked", "pending", "failed", "unknown"}
var psetTrans = []string{"linked", "pending", "missing"}

// 状态转移表
// 维度: [当前状态][收到的邻居关系] -> 下一状态
var helloTrans = [4][3]uint32{
	//              TRANS_LINKED, TRANS_PENDING, TRANS_MISSING
	/* PSET_LINKED */ {PSET_LINKED, PSET_LINKED, PSET_FAILED},
	/* PSET_PENDING */ {PSET_LINKED, PSET_LINKED, PSET_PENDING},
	/* PSET_FAILED */ {PSET_FAILED, PSET_PENDING, PSET_PENDING},
	// /* PSET_UNKNOWN */ {PSET_LINKED, PSET_LINKED, PSET_PENDING},
	/* PSET_UNKNOWN */ {PSET_FAILED, PSET_LINKED, PSET_PENDING},
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
	me := psm.ownerNode
	// log.Printf("Node %d: started to receive Hello Msg for updating Pset state", me.ID)

	// TODO：是否要加锁以安全地访问和修改PSetManager的状态
	// 使用 for-range 循环不断地从channel中接收任务
	for tmp := range psm.psetStateUpdateChan {
		curState, _ := me.PsetManager.GetStatus(tmp.node)
		nextState := helloTrans[curState][tmp.trans]
		curActive, _ := me.PsetManager.GetActive(tmp.node)

		log.Printf("Node %d: Pset update for Node %d: %s[%s] ==> %s",
			me.ID, tmp.node, psetStates[curState], psetTrans[tmp.trans], psetStates[nextState])

		if curState == PSET_UNKNOWN {
			me.PsetManager.Add(tmp.node, nextState, tmp.active)
			psm.Update()
		} else if curState != nextState || curActive != tmp.active {
			me.PsetManager.Update(tmp.node, nextState, tmp.active)
			psm.Update()
		}

		// 检查是否需要为新节点发起setup_req
		if !me.Active && tmp.active && nextState == PSET_LINKED {
			log.Printf("Node %d: New Active/linked neighbor %d found. Sending setup_req to self via proxy %d.", me.ID, tmp.node, tmp.node)
			// todo: 参数对应
			me.SendSetupReq(me.ID, me.ID, 0, me.ID, tmp.node, nil)
		}
	}
	log.Printf("Node %d: ended to receive Hello Msg for updating Pset state", me.ID)
}

// Update 更新 PsetState 快照（从 PSetManager 同步数据）
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
