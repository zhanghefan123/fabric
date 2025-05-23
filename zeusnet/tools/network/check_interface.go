//go:build linux

package network

import (
	"fmt"
	"net"
	"time"
)

// IsNetworkInterfaceUp 用来进行检查接口是否UP并且被设置了IP地址
// @param interfaceName 接口的名称
// @param listenIpAddress 这个接口需要是这个地址
// @return 是否可以监听这个地址了
func IsNetworkInterfaceUp(interfaceName, listenIpAddr string) (bool, error) {
	// 1. 获取所有的网络接口
	interfaces, err := net.Interfaces()
	if err != nil {
		return false, fmt.Errorf("cannot get network interfaces: %v", err)
	}
	// 2. 进行所有的接口的遍历
	for _, networkIntf := range interfaces {
		// 2.1 判断是否是我们需要的接口
		if networkIntf.Name == interfaceName {
			// 2.2 判断这个接口是否是UP的
			if networkIntf.Flags&net.FlagUp == 0 {
				return false, fmt.Errorf("network interface %s is down", interfaceName)
			}
			// 2.3 判断这个接口是否有IP地址
			var addrs []net.Addr
			addrs, err = networkIntf.Addrs()
			if err != nil {
				return false, fmt.Errorf("cannot get addresses for network interface %s: %v", interfaceName, err)
			}
			// 2.4 遍历所有的 ip 地址看是否有我们需要的地址
			for _, addr := range addrs {
				// 2.4.1 判断是否是我们需要的地址
				if ipNet, ok := addr.(*net.IPNet); ok {
					if ipNet.IP.String() == listenIpAddr {
						return true, nil
					}
				}
			}
		} else {
			continue
		}
	}
	return false, fmt.Errorf("network interface %s is not found", interfaceName)
}

// SleepUntilInterfaceUp 循环检查一个接口是否存在, 如果这个接口存在并且 up, 就进行返回
func SleepUntilInterfaceUp(interfaceName, listenIpAddr string) error {
	timer := time.NewTicker(time.Second)
	defer timer.Stop()
	for {
		select {
		case <-timer.C:
			fmt.Println("check interface")
			networkInterfaceUp, _ := IsNetworkInterfaceUp(interfaceName, listenIpAddr)
			if networkInterfaceUp {
				fmt.Printf("network interface %s is up\n", interfaceName)
				return nil
			}
		}
	}
}
