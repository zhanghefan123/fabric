package system

import (
	"github.com/hyperledger/fabric/common/fabhttp"
	"github.com/hyperledger/fabric/core/operations"
	"github.com/hyperledger/fabric/internal/pkg/comm"
)

var (
	ParameterInstance = &Parameter{}
)

type Parameter struct {
	OpsSystem         *operations.System
	AdminServer       *fabhttp.Server
	GrpcServer        *comm.GRPCServer
	ClusterGRPCServer *comm.GRPCServer
}
