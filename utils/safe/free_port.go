package safe

import (
	"fmt"
	"net"
	"sync"
)

// PortAllocator 线程安全的端口分配器
type PortAllocator struct {
	mu       sync.Mutex
	reserved map[int]struct{} // 已预留的端口集合
}

// NewPortAllocator 创建新的端口分配器
func NewPortAllocator() *PortAllocator {
	return &PortAllocator{
		reserved: make(map[int]struct{}),
	}
}

// Allocate 在指定范围内分配一个空闲端口
// 该端口会被立即标记为预留，其他调用不会获得相同端口
func (pa *PortAllocator) Allocate(startPort, maxRange int) (int, error) {
	pa.mu.Lock()
	defer pa.mu.Unlock()

	for port := startPort; port < startPort+maxRange; port++ {
		// 检查是否已被本分配器预留
		if _, exists := pa.reserved[port]; exists {
			continue
		}

		// 检查系统层面是否可用
		if !pa.isPortFree(port) {
			continue
		}

		// 预留该端口
		pa.reserved[port] = struct{}{}
		return port, nil
	}

	return 0, fmt.Errorf("no free port found in range %d-%d", startPort, startPort+maxRange-1)
}

// Release 释放指定的端口，使其可以被重新分配
func (pa *PortAllocator) Release(port int) {
	pa.mu.Lock()
	defer pa.mu.Unlock()
	delete(pa.reserved, port)
}

// isPortFree 检查系统层面端口是否可用
func (pa *PortAllocator) isPortFree(port int) bool {
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return false
	}
	listener.Close()
	return true
}

// 全局单例
var (
	globalAllocator     *PortAllocator
	globalAllocatorOnce sync.Once
)

// GetGlobalPortAllocator 获取全局端口分配器（懒加载）
func GetGlobalPortAllocator() *PortAllocator {
	globalAllocatorOnce.Do(func() {
		globalAllocator = NewPortAllocator()
	})
	return globalAllocator
}

// 包级便捷函数（对标 Python 版本）

// GetFreePort 获取一个空闲端口（线程安全）
func GetFreePort(startPort int, maxRange int) (int, error) {
	return GetGlobalPortAllocator().Allocate(startPort, maxRange)
}

// ReleasePort 释放指定的端口
func ReleasePort(port int) {
	GetGlobalPortAllocator().Release(port)
}
