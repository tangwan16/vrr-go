package vrr

import (
	"log"
)

const (
	// 消息类型
	VRR_HELLO      = 0x1
	VRR_SETUP_REQ  = 0x2
	VRR_SETUP      = 0x3
	VRR_SETUP_FAIL = 0x4
	VRR_TEARDOWN   = 0x5
	VRR_DATA       = 0x6
)

// --- 节点消息处理器 ---
// ProcessMessage 是节点的消息处理入口点
func (n *Node) rcvMessage(msg Message) {
	log.Printf("Node %d: Received message type %d from %d to %d", n.ID, msg.Type, msg.Src, msg.Dst)

	// 重置相关超时和失败计数
	n.ResetActiveTimeout()
	n.PsetManager.ResetFailCount(msg.Src)

	// 根据消息类型分发处理
	switch msg.Type {
	case VRR_HELLO:
		n.receiveHello(msg)
	case VRR_SETUP_REQ:
		n.receiveSetupReq(msg)
	case VRR_SETUP:
		n.receiveSetup(msg)
	case VRR_SETUP_FAIL:
		n.receiveSetupFail(msg)
	case VRR_TEARDOWN:
		n.receiveTeardown(msg)
	case VRR_DATA:
		n.receiveData(msg)
	default:
		log.Printf("Node %d: Unknown message type: %d", n.ID, msg.Type)
	}
}

// --- 各类型消息处理函数 ---
// receiveData 处理数据消息
func (n *Node) receiveData(msg Message) {
	log.Printf("Node %d: Handling DATA message from %d to %d", n.ID, msg.Src, msg.Dst)

	if msg.Dst == n.ID {
		// 数据包到达目的地
		log.Printf("Node %d: Data packet delivered from %d, payload size: %d",
			n.ID, msg.Src, len(msg.Payload))
		// TODO: 递交给上层应用
	} else {
		// 需要转发
		nextHop := n.RoutingTable.GetNext(msg.Dst)
		if nextHop == 0 {
			log.Printf("Node %d: No route to forward data to %d", n.ID, msg.Dst)
			return
		}

		// 更新NextHop并转发
		msg.NextHop = nextHop
		n.Network.Send(msg)
		log.Printf("Node %d: Forwarded data to %d via %d", n.ID, msg.Dst, nextHop)
	}
}

// receiveHello 处理Hello消息
func (n *Node) receiveHello(msg Message) {
	src := msg.Src
	trans := TRANS_MISSING // 默认是 MISSING
	me := n
	actitve := msg.SenderActive

	log.Printf("Node %d: Handling HELLO message from %d", n.ID, msg.Src)

	if len(msg.HelloInfoLinkActive) > VRR_PSET_SIZE && len(msg.HelloInfoLinkNotActive) > VRR_PSET_SIZE && len(msg.HelloInfoPending) > VRR_PSET_SIZE {
		log.Printf("Node %d: Invalid HELLO message, empty HelloInfo", n.ID)
		return
	}

	// 检查自己是否在发送者的物理邻居集中
	for _, nodeID := range msg.HelloInfoLinkActive {
		if nodeID == me.ID {
			trans = TRANS_LINKED
			break
		}
	}

	// 检查自己是否在发送者的非活跃邻居集中
	for _, nodeID := range msg.HelloInfoLinkNotActive {
		if nodeID == me.ID {
			trans = TRANS_MISSING
			break
		}
	}

	// 检查自己是否在发送者的待定邻居集中
	for _, nodeID := range msg.HelloInfoPending {
		if nodeID == n.ID {
			trans = TRANS_PENDING
			break
		}
	}
	update := PsetStateUpdate{
		node:   src,
		trans:  trans,
		Active: actitve,
	}
	// 将任务交给PsetStateManager的工作队列
	n.PsetStateManager.ScheduleUpdate(update)
}

// --------------------public api-----------------------------
// resetActiveTimeout 重置活跃超时
func (n *Node) resetActiveTimeout() {
	n.lock.Lock()
	defer n.lock.Unlock()
	n.Timeout = 0 // 重置超时计数器
}

// setActive 设置节点活跃状态
func (n *Node) setActive(Active bool) {
	n.lock.Lock()
	defer n.lock.Unlock()
	n.Active = Active
}

// ------------------Vrr 论文实现方法------------------
/* Receive (<setup_req,src,dst,proxy,vset'>, sender)
   nh := NextHopExclude(rt, dst, src)
   if (nh != null)
       Send <setup_req, src, dst, proxy, vset’> to nh
   else
       ovset := vset;
	   added := Add(vset, src, vset’)
       if (added)
           Send <setup, me, src, NewPid(),proxy, ovset> to me
       else
           Send <setup_fail, me, src, proxy, ovset> to me */
// handleSetupReq 处理Setup请求消息
// to do：缺乏 vset' 的处理
func (n *Node) receiveSetupReq(msg Message) {
	log.Printf("Node %d: Receiving SETUP_REQ from %d to %d via proxy %d", n.ID, msg.Src, msg.Dst, msg.Proxy)

	// 确定下一跳，排除消息发送者
	nextHop := n.RoutingTable.GetNextExclude(msg.Dst, msg.Src)
	// 本节点是src到dst的中间节点
	if nextHop != 0 {
		log.Printf("Node %d: Forwarding SETUP_REQ to next hop %d", n.ID, nextHop)
		n.SendSetupReq(msg.Src, msg.Dst, msg.Proxy, msg.Vset_, nextHop)
		return
	} else {
		// 本节点就是dst
		myVset := n.VsetManager.GetAll()
		if n.AddMsgSrcToLocalVset(msg.Src, msg.Vset_) {
			// 从自己开始setup
			n.SendSetup(n.ID, msg.Src, n.newPathID(), msg.Proxy, myVset, n.ID, n.ID)
		} else {
			// 添加失败，发送Setup失败消息
			n.SendSetupFail(n.ID, msg.Src, msg.Proxy, myVset, n.ID)
		}
	}
}

/*
Receive (<setup,src,dst,proxy,vset'>, sender)
    nh := (dst in pset) ? dst : NextHop(rt, proxy)
    added := Add(rt, src, dst, sender, nh, pid )
    if (¬added ∨ sender ！∈ pset)
        TearDownPath( pid, src , sender)
    else if (nh = null)
        Send setup, src, dst, pid, proxy, vset’ to nh
    else if (dst = me)
        added := Add(vset, src, vset’)
        if (¬added)
            TearDownPath( pid, src , null)
    else
        TearDownPath( pid, src , null)
*/
// receiveSetup 处理Setup消息
func (n *Node) receiveSetup(msg Message) {
	log.Printf("Node %d: Receiving SETUP from %d to %d, pathID %d, proxy %d,sender %d", n.ID, msg.Src, msg.Dst, msg.Pid, msg.Proxy, msg.Sender)

	// 确定下一跳
	var nextHop uint32

	if n.PsetManager.IsActiveLinkedPset(msg.Dst) {
		nextHop = msg.Dst
	} else {
		nextHop = n.RoutingTable.GetNext(msg.Proxy)
	}
	addedToRoute := n.RoutingTable.AddRoute(msg.Src, msg.Dst, msg.Sender, nextHop, msg.Pid)

	// 添加路由条目
	if !addedToRoute || !n.PsetManager.IsActiveLinkedPset(msg.Sender) {
		log.Printf("Node %d: Couldn't add route, tearing down path to %d", n.ID, msg.Src)
		// 故障！要么路由添加失败，要么发送者不再是我的邻居
		n.RoutingTable.TearDownPath(msg.Pid, msg.Src, msg.Sender)
		return
	}

	// 若还有下一跳，则继续转发 setup
	if nextHop != 0 {
		n.SendSetup(msg.Src, msg.Dst, msg.Pid, msg.Proxy, msg.Vset_, nextHop, n.ID)
		return
	}

	// 本节点就是dst
	if msg.Dst == n.ID {
		addedToVset := n.AddMsgSrcToLocalVset(msg.Src, msg.Vset_)
		if !addedToVset {
			log.Printf("Node %d: Couldn't add %d to vset, tearing down path", n.ID, msg.Src)
			//  路径本身是好的，但我（目标节点）由于某种策略无法将源节点加入我的vset
			// 这是一个“逻辑拒绝”，而不是“链路错误”
			n.RoutingTable.TearDownPath(msg.Pid, msg.Src, 0)
		}
		return
	}

	// 异常情况：无下一跳且目标不是我
	log.Printf("Node %d: Unexpected setup condition, tearing down path to %d", n.ID, msg.Src)
	n.RoutingTable.TearDownPath(msg.Pid, msg.Src, 0)

}

/*
Receive (<teardown, <pid,ea>, vset‘>, sender)
    < ea , eb , na , nb , pid := Remove(rt, <pid, ea>)
    next := (sender = na ) ? nb : na
    if (next != null)
        Send <teardown, <pid, ea> , vset’ >to next
    else
        e := (sender = na ) ? eb : ea
        Remove(vset, e)
        if (vset’ = null)
            Add(vset, null, vset’)
        else
            proxy := PickRandomActive(pset)
            Send <setup_req, me, e, proxy, vset> to proxy
*/
// receiveTeardown 处理Teardown消息
func (n *Node) receiveTeardown(msg Message) {
	log.Printf("Node %d: Receiving TEARDOWN pathID %d", n.ID, msg.Pid)

	route := n.RoutingTable.RemoveRoute(msg.Pid, msg.Endpoint)

	// 确定下一个要发送teardown的节点，到达ea或eb时，next=0
	var next uint32
	if msg.Sender == route.Na {
		next = route.Nb
	} else {
		next = route.Na
	}

	if next != 0 {
		// ea 和 eb中间节点
		n.SendTeardown(msg.Pid, msg.Endpoint, msg.Vset_, next)
	} else {
		// 到达ea或eb节点，更新本地vset
		var e uint32
		if msg.Sender == route.Na {
			e = route.Eb
		} else {
			e = route.Ea
		}
		n.VsetManager.Remove(e)

		// 如果vset'不为空
		if len(msg.Vset_) > 0 {
			// 合并vset'到本地vset
			n.AddMsgSrcToLocalVset(0, msg.Vset_)
		} else {
			//
			// vset'为空，发生了链路错误，通过其他代码重新建立连接
			proxy, _ := n.PsetManager.GetProxy()
			myVset := n.VsetManager.GetAll()
			n.SendSetupReq(n.ID, e, proxy, myVset, proxy)
		}
	}
}

/*
Receive (<setup_fail,src,dst,proxy,vset'>, sender)
    nh := (dst ∈ pset) ? dst : NextHop(rt, proxy)
    if (nh = null)
        Send <setup_fail, src, dst, proxy, vset’> to nh
    else if (dst = me)
        Add(vset, null, vset’ Union {src} )
*/
// receiveSetupFail 处理Setup失败消息
func (n *Node) receiveSetupFail(msg Message) {
	log.Printf("Node %d: Handling SETUP_FAIL from %d to %d via proxy %d",
		n.ID, msg.Src, msg.Dst, msg.Proxy)

	// 确定下一跳
	var nextHop uint32

	if n.PsetManager.IsActiveLinkedPset(msg.Dst) {
		nextHop = msg.Dst
	} else {
		nextHop = n.RoutingTable.GetNext(msg.Proxy)
	}

	if nextHop != 0 {
		// 转发Setup失败消息
		n.SendSetupFail(msg.Src, msg.Dst, msg.Proxy, msg.Vset_, nextHop)
	} else if msg.Dst == n.ID {
		// 自己是目的地，将src添加到vset并处理
		srcVsetWithSrc := append(msg.Vset_, msg.Src)
		n.AddMsgSrcToLocalVset(0, srcVsetWithSrc)
	}
}
