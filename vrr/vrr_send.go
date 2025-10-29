package vrr

import "log"

// SendSetupReq 构建并发送一个 setup request 数据包
func (n *Node) SendSetupReq(src, dest, sender, nextHop uint32, proxy uint32, vset_ []uint32) bool {
	log.Printf("src %d: SendSetupReq to dest=%d via proxy=%d", src, dest, proxy)

	// 2. 创建消息信封 (Message)，并装入 Payload
	msg := Message{
		Type:    VRR_SETUP_REQ,
		Src:     src,
		Dst:     dest,
		Sender:  sender,
		NextHop: nextHop,

		Payload: &SetupReqPayload{
			Proxy: proxy,
			Vset_: vset_,
		},
	}

	n.Network.Send(msg)
	return true
}

// SendSetup 构建并发送一个 setup 数据包
func (n *Node) SendSetup(src, dest, sender uint32, nextHop uint32, pid, proxy uint32, vset []uint32) bool {
	log.Printf("Node %d: SendSetup src=%d dest=%d pathID=%d proxy=%d nextHop=%d",
		n.ID, src, dest, pid, proxy, nextHop)

	msg := Message{
		Type:    VRR_SETUP,
		Src:     src,
		Dst:     dest,
		Sender:  sender,  // 设置实际发送者为上一跳
		NextHop: nextHop, // 消息发送给 nextHop

		Payload: &SetupPayload{
			Pid:   pid,
			Proxy: proxy,
			Vset_: append([]uint32(nil), vset...), // 复制切片
		},
	}

	n.Network.Send(msg)
	return true
}

// SendSetupFail 构建并发送一个 setup fail 数据包
func (n *Node) SendSetupFail(src, dst, sender, nextHop, proxy uint32, vset []uint32) bool {
	log.Printf("Node %d: SendSetupFail src=%d dst=%d proxy=%d nextHop=%d",
		n.ID, src, dst, proxy, nextHop)

	msg := Message{
		Type:    VRR_SETUP_FAIL,
		Src:     src,
		Dst:     dst,
		Sender:  sender,
		NextHop: nextHop,

		Payload: &SetupFailPayload{
			Proxy: proxy,
			Vset_: append([]uint32(nil), vset...), // 复制切片
		},
	}

	n.Network.Send(msg)
	return true
}

// SendTeardown 构建并发送一个 teardown 数据包
func (n *Node) SendTeardown(pathID, endpoint uint32, vset_ []uint32, nextHop uint32) bool {
	log.Printf("Node %d: SendTeardown pathID=%d endpoint=%d nextHop=%d",
		n.ID, pathID, endpoint, nextHop)

	msg := Message{
		Type:    VRR_TEARDOWN,
		Src:     n.ID, // Teardown 消息由当前节点发起
		Dst:     0,    // 通常是广播或沿路径反向传播，具体取决于协议
		Sender:  n.ID,
		NextHop: nextHop,

		Payload: &TeardownPayload{
			Pid:      pathID,
			Endpoint: endpoint,
			Vset_:    append([]uint32(nil), vset_...), // 复制切片
		},
	}

	n.Network.Send(msg)
	return true
}

// SendHello 构建并发送一个 hello 数据包（广播）
func (n *Node) SendHello() bool {
	// log.Printf("Node %d: SendHelloPkt (broadcasting)", n.ID)

	// 更新 psetState 快照
	n.PsetStateManager.Update()

	msg := Message{
		Type:    VRR_HELLO,
		Src:     n.ID,
		Dst:     0, // 广播地址
		Sender:  n.ID,
		NextHop: 0, // 广播，无需指定下一跳
		Payload: &HelloPayload{
			SenderActive:           n.Active,
			HelloInfoLinkActive:    append([]uint32(nil), n.PsetStateManager.LinkActive...),
			HelloInfoLinkNotActive: append([]uint32(nil), n.PsetStateManager.LinkNotActive...),
			HelloInfoPending:       append([]uint32(nil), n.PsetStateManager.Pending...),
		},
	}

	n.Network.Send(msg)
	return true
}

// SendData 发送数据消息
func (n *Node) SendData(dest uint32, data []byte) bool {
	// 查找路由
	nextHop := n.RoutingTable.GetNext(dest)
	if nextHop == 0 {
		log.Printf("Node %d: No route to destination %d", n.ID, dest)
		return false
	}

	log.Printf("Node %d: SendData to dest=%d via nextHop=%d", n.ID, dest, nextHop)

	msg := Message{
		Type:    VRR_DATA,
		Src:     n.ID,
		Dst:     dest,
		Sender:  n.ID,
		NextHop: nextHop,
		Payload: &DataPayload{
			Data: append([]byte(nil), data...),
		},
	}

	n.Network.Send(msg)
	return true
}

// NewPid 作为 Node 的方法生成一个随机的 32 位路径 ID
// 确保生成的 ID 不与当前节点的 vset 中的任何节点 ID 冲突
func (n *Node) NewPid() uint32 {
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
