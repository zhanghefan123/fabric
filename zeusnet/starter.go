//go:build linux

package zeusnet

import (
	"fmt"
	"github.com/hyperledger/fabric/zeusnet/modules/config"
	"github.com/hyperledger/fabric/zeusnet/modules/frr"
	"github.com/hyperledger/fabric/zeusnet/tools/network"
)

func Start() error {
	// 1. 进行配置文件的加载
	err := config.EnvLoaderInstance.LoadEnv()
	if err != nil {
		return fmt.Errorf("load env error: %v", err)
	}
	fmt.Println(config.EnvLoaderInstance)
	// 3. 等待接口启动
	err = network.SleepUntilInterfaceUp(config.EnvLoaderInstance.FirstInterfaceName, config.EnvLoaderInstance.FirstInterfaceAddr)
	if err != nil {
		return fmt.Errorf("return until interface up error: %v", err)
	}
	// 4. frr 的启动程序
	err = frr.StartFrr()
	if err != nil {
		return fmt.Errorf("start frr failed: %w", err)
	}
	return nil
}
