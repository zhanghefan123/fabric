package zeusnet

import (
	"fmt"
	"github.com/hyperledger/fabric/zeusnet/modules/config"
	"github.com/hyperledger/fabric/zeusnet/modules/frr"
)

var (
	EnvLoaderInstance *config.EnvLoader
)

func Start() error {
	err := frr.StartFrr()
	if err != nil {
		return fmt.Errorf("start frr failed: %w", err)
	} else {
		return nil
	}
}
