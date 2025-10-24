package vrr

type Networker interface {
	Send(msg Message) // 假设 Message 类型也在 vrr 包或其依赖的包中
}
