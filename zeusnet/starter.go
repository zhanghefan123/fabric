//go:build linux

package zeusnet

import (
	"fmt"
	"github.com/hyperledger/fabric/zeusnet/modules/frr"
	"github.com/hyperledger/fabric/zeusnet/tools/network"
	"github.com/hyperledger/fabric/zeusnet/variables"
)

func Start() error {
	// 1. 进行配置文件的加载
	err := variables.EnvLoaderInstance.LoadEnv()
	if err != nil {
		return fmt.Errorf("load env error: %v", err)
	}
	fmt.Println(variables.EnvLoaderInstance.FirstInterfaceName, variables.EnvLoaderInstance.FirstInterfaceAddr)
	err = network.SleepUntilInterfaceUp(variables.EnvLoaderInstance.FirstInterfaceName, variables.EnvLoaderInstance.FirstInterfaceAddr)
	if err != nil {
		return fmt.Errorf("return until interface up error: %v", err)
	}
	// 2. frr 的启动程序
	err = frr.StartFrr()
	if err != nil {
		return fmt.Errorf("start frr failed: %w", err)
	}
	// 3. 等待第一个接口启动了再启动
	return nil
}
