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
	msgType := GetMessageTypeString(msg.Type)
	if msgType == "VRR_HELLO" {
		// 处理 VRR_HELLO 消息
		log.Printf("Node %d: Received periodic HELLO Msg from Node %d in same subnet", n.ID, msg.Src)

	} else {
		log.Printf("Node %d: Received message type %s from Node %d to Node %d", n.ID, msgType, msg.Src, msg.Dst)
	}

	// 重置参数
	n.ResetActiveTimeout()
	// to do:为什么重置失败计数的是msg.Src？而不是msg.Sender？
	n.PsetManager.ResetFailCount(msg.Src)

	// 根据消息类型分发处理
	switch msg.Type {
	case VRR_HELLO:
		// 使用类型断言获取具体的 Payload
		if payload, ok := msg.Payload.(*HelloPayload); ok {
			n.receiveHello(msg, payload)
		} else {
			log.Printf("Node %d: Invalid payload for HELLO message", n.ID)
		}
	case VRR_SETUP_REQ:
		if payload, ok := msg.Payload.(*SetupReqPayload); ok {
			n.receiveSetupReq(msg, payload)
		} else {
			log.Printf("Node %d: Invalid payload for SETUP_REQ message", n.ID)
		}
	case VRR_SETUP:
		if payload, ok := msg.Payload.(*SetupPayload); ok {
			n.receiveSetup(msg, payload)
		} else {
			log.Printf("Node %d: Invalid payload for SETUP message", n.ID)
		}
	case VRR_SETUP_FAIL:
		if payload, ok := msg.Payload.(*SetupFailPayload); ok {
			n.receiveSetupFail(msg, payload)
		} else {
			log.Printf("Node %d: Invalid payload for SETUP_FAIL message", n.ID)
		}
	case VRR_TEARDOWN:
		if payload, ok := msg.Payload.(*TeardownPayload); ok {
			n.receiveTeardown(msg, payload)
		} else {
			log.Printf("Node %d: Invalid payload for TEARDOWN message", n.ID)
		}
	case VRR_DATA:
		if payload, ok := msg.Payload.(*DataPayload); ok {
			n.receiveData(msg, payload)
		} else {
			log.Printf("Node %d: Invalid payload for DATA message", n.ID)
		}
	default:
		log.Printf("Node %d: Unknown message type: %s", n.ID, GetMessageTypeString(msg.Type))
	}
}

// --- 各类型消息处理函数 ---
// receiveData 处理数据消息
func (n *Node) receiveData(msg Message, payload *DataPayload) {
	if msg.Dst == n.ID {
		// 数据包到达目的地
		log.Printf("Node %d: Data packet delivered from %d, payload size: %d",
			n.ID, msg.Src, len(payload.Data))
		// TODO: 递交给上层应用|Data处理逻辑
	} else {
		nextHop := n.RoutingTable.GetNext(msg.Dst)
		if nextHop == 0 {
			log.Printf("Node %d: No route to forward data to %d", n.ID, msg.Dst)
			return
		}

		// 转发DataMsg
		msg.NextHop = nextHop
		n.Network.Send(msg)

		log.Printf("Node %d: Forwarded data to %d via %d", n.ID, msg.Dst, nextHop)
	}
}

// ------------------Vrr 论文实现方法------------------
// receiveHello 处理Hello消息
func (n *Node) receiveHello(msg Message, payload *HelloPayload) {
	if len(payload.HelloInfoLinkActive) > VRR_PSET_SIZE || len(payload.HelloInfoLinkNotActive) > VRR_PSET_SIZE || len(payload.HelloInfoPending) > VRR_PSET_SIZE {
		log.Printf("Node %d: Invalid HelloInfo Size.Dropping packet.", n.ID)
		return
	}
	// 解析hello消息的路由消息内容
	src := msg.Src
	// 解析HELLO 元数据消息内容
	active := payload.SenderActive
	linkActive := payload.HelloInfoLinkActive
	linkNotActive := payload.HelloInfoLinkNotActive
	pending := payload.HelloInfoPending

	trans := TRANS_MISSING // 默认是 MISSING
	me := n

	// 检查自己是否在发送者的物理邻居集中
	for _, nodeID := range linkActive {
		if nodeID == me.ID {
			trans = TRANS_LINKED
			break
		}
	}

	// 检查自己是否在发送者的非活跃邻居集中
	for _, nodeID := range linkNotActive {
		if nodeID == me.ID {
			trans = TRANS_LINKED
			break
		}
	}

	// 检查自己是否在发送者的待定邻居集中
	for _, nodeID := range pending {
		if nodeID == n.ID {
			trans = TRANS_PENDING
			break
		}
	}
	update := PsetStateUpdate{
		node:   src,
		trans:  trans,
		active: active,
	}
	// 将任务交给PsetStateManager的工作队列
	n.PsetStateManager.ScheduleUpdate(update)
}

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
func (n *Node) receiveSetupReq(msg Message, payload *SetupReqPayload) {
	// 解析消息的路由消息
	src := msg.Src
	dst := msg.Dst

	// 解析消息的元数据信息
	proxy := payload.Proxy
	vset_ := payload.Vset_

	me := n.ID

	// 确定下一跳，排除消息发送者
	nextHop := n.RoutingTable.GetNextExclude(dst, src)

	// 本节点是src到dst的中间节点
	if nextHop != 0 {
		log.Printf("Node %d: Forwarding SETUP_REQ to next hop %d", me, nextHop)
		// 转发SetupReq消息给nextHop
		n.SendSetupReq(src, dst, me, nextHop, proxy, vset_)
		return
	} else {
		// 本节点就是dst或最接近dst的节点

		vset := n.VsetManager.GetAll()
		added := n.Add(vset, src, vset_)
		if added {
			// 从自己开始setup
			// n.SendSetup(me, src, me, me, , proxy, vset)
			n.LocalRcvSetup(src, n.NewPid(), proxy, vset)
		} else {
			// 添加失败，发送Setup失败消息
			n.SendSetupFail(me, src, me, me, proxy, vset)
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
func (n *Node) receiveSetup(msg Message, payload *SetupPayload) {
	me := n.ID
	// 解析消息的路由信息
	src := msg.Src
	dst := msg.Dst
	sender := msg.Sender

	// 解析消息的元数据信息
	pid := payload.Pid
	proxy := payload.Proxy
	vset_ := payload.Vset_

	if sender == me {
		log.Printf("Node %d: Received setup from myself", me)
	} else {
		inPset := n.PsetManager.GetStatus(sender)
		if inPset == PSET_UNKNOWN {
			n.RoutingTable.TearDownPath(pid, src, sender)
			log.Printf("Node %d: Sender %d is not in pset!", me, sender)
		}

	}

	// 确定下一跳
	var nextHop uint32
	if n.PsetManager.GetStatus(dst) == PSET_UNKNOWN {
		if dst == me {
			nextHop = 0
		} else {
			nextHop = n.RoutingTable.GetNext(proxy)
		}
	} else {
		// dst 在 pset 中
		nextHop = dst
	}

	added := n.RoutingTable.Add(src, dst, sender, nextHop, pid)
	if !added {
		// to do:明确是写sender还是null
		n.RoutingTable.TearDownPath(pid, src, 0)
		log.Printf("Node %d: Couldn't add route, tearing down path to %d", me, src)
	}

	// 转发Setup消息给nexthop
	if nextHop != 0 {
		n.SendSetup(src, dst, me, nextHop, pid, proxy, vset_)
		return
	}
	// 本节点就是dst
	vset := n.VsetManager.GetAll()
	add := n.Add(vset, src, vset_)

	if add {
		log.Printf("Node %d: vset-paths established by setup message from %d", me, src)
		n.Active = true
		return
	} else {
		log.Printf("Node %d: Couldn't add %d to vset, tearing down path", me, src)
		//  路径本身是好的，但我（目标节点）由于某种策略无法将源节点加入我的vset
		// 这是一个“逻辑拒绝”，而不是“链路错误”
	}

	// 异常情况：无下一跳且目标不是我
	n.RoutingTable.TearDownPath(pid, src, 0)
	return
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
func (n *Node) receiveTeardown(msg Message, payload *TeardownPayload) {

	route := n.RoutingTable.RemoveRoute(payload.Pid, payload.Endpoint)

	// 确定下一个要发送teardown的节点，到达ea或eb时，next=0
	var nextHop uint32
	if msg.Sender == route.Na {
		nextHop = route.Nb
	} else {
		nextHop = route.Na
	}

	if nextHop != 0 {
		// ea 和 eb中间节点
		// 1. 更新消息信封的路由信息
		msg.Sender = n.ID
		msg.NextHop = nextHop
		// 2. 直接将修改后的消息发送出去
		n.Network.Send(msg)
		// n.SendTeardown(payload.Pid, payload.Endpoint, payload.Vset_, next)
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
		if len(payload.Vset_) > 0 {
			vset := n.VsetManager.GetAll()
			// 合并vset'到本地vset
			n.Add(vset, 0, payload.Vset_)
		} else {
			//
			// vset'为空，发生了链路错误，通过其他代码重新建立连接
			// proxy, _ := n.PsetManager.GetProxy()
			// n.SendSetupReq(n.ID, e, proxy)
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
func (n *Node) receiveSetupFail(msg Message, payload *SetupFailPayload) {
	// 确定下一跳
	var nextHop uint32

	if n.PsetManager.IsActiveLinkedPset(msg.Dst) {
		nextHop = msg.Dst
	} else {
		nextHop = n.RoutingTable.GetNext(payload.Proxy)
	}

	if nextHop != 0 {
		// 转发Setup失败消息
		// 1、更新消息信封的路由信息并转发
		msg.Sender = n.ID
		msg.NextHop = nextHop
		n.Network.Send(msg)
		// n.SendSetupFail(msg.Src, msg.Dst, n.ID, nextHop, payload.Proxy, payload.Vset_)
	} else if msg.Dst == n.ID {
		// 自己是目的地，将src添加到vset并处理
		srcVsetWithSrc := append(payload.Vset_, msg.Src)
		vset := n.VsetManager.GetAll()
		n.Add(vset, 0, srcVsetWithSrc)
	}
}

func (n *Node) LocalRcvSetup(dst, pid, proxy uint32, vset_ []uint32) {
	me := n.ID
	// 确定下一跳
	var nextHop uint32
	if n.PsetManager.GetStatus(dst) == PSET_UNKNOWN {
		nextHop = n.RoutingTable.GetNext(proxy)
	} else {
		nextHop = dst
	}

	added := n.RoutingTable.Add(me, dst, 0, nextHop, pid)
	if !added {
		// to do:明确是写sender还是null
		n.RoutingTable.TearDownPath(pid, me, 0)
		log.Printf("Node %d: Couldn't add route, tearing down path to %d", me, me)
	}

	// 转发Setup消息给nexthop
	if nextHop != 0 {
		n.SendSetup(me, dst, me, nextHop, pid, proxy, vset_)
		return
	}
	return
}
