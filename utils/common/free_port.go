package common

import "net"

// 获得一个随机可用端口，但是不确保会不会被抢

func GetFreePort() (int, error) {
	// 绑定到 :0 表示让系统分配随机可用端口
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer listener.Close()

	// 提取分配的端口号
	addr := listener.Addr().(*net.TCPAddr)
	return addr.Port, nil
}
