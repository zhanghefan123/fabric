package zeusnet

import (
	"fmt"
	"github.com/hyperledger/fabric/zeusnet/modules/frr"
	"github.com/hyperledger/fabric/zeusnet/variables"
)

func Start() error {
	// 1. 进行配置文件的加载
	variables.EnvLoaderInstance.LoadEnv()
	// 2. 进行 frr 的读取
	err := frr.StartFrr()
	if err != nil {
		return fmt.Errorf("start frr failed: %w", err)
	} else {
		return nil
	}
}
