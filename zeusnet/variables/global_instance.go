package variables

import (
	"github.com/hyperledger/fabric/zeusnet/modules/config"
	"github.com/hyperledger/fabric/zeusnet/modules/system"
)

var (
	EnvLoaderInstance *config.EnvLoader = &config.EnvLoader{}
	ParameterInstance *system.Parameter = &system.Parameter{}
)
