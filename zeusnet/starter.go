package zeusnet

import (
	"fmt"
	"github.com/hyperledger/fabric/zeusnet/modules/frr"
	"github.com/hyperledger/fabric/zeusnet/variables"
)

func Start() error {
	// 1. 进行配置文件的加载
	variables.EnvLoaderInstance.LoadEnv()
	// 2. 判断可能的情况
	if variables.EnvLoaderInstance.IsAllVariablesNone() {
		// 2.1 标准的启动程序
		return nil
	} else {
		// 2.2 frr 的启动程序
		err := frr.StartFrr()
		if err != nil {
			return fmt.Errorf("start frr failed: %w", err)
		} else {
			return nil
		}
	}
}
