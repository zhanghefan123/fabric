package apis

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/hyperledger/fabric/zeusnet/variables"
)

// StopFabric 停止 Orderer 节点
func StopFabric(c *gin.Context) {
	err := variables.ParameterInstance.OpsSystem.Stop()
	if err != nil {
		c.JSON(500, gin.H{
			"message": "stop fabric error: " + err.Error(),
		})
		fmt.Println("stop fabric error: ", err.Error())
		return
	}
	err = variables.ParameterInstance.AdminServer.Stop()
	if err != nil {
		c.JSON(500, gin.H{
			"message": "stop fabric admin server error: " + err.Error(),
		})
		fmt.Println("stop fabric admin server error: ", err.Error())
		return
	}
	variables.ParameterInstance.GrpcServer.Stop()
	if variables.ParameterInstance.ClusterGRPCServer != nil {
		variables.ParameterInstance.ClusterGRPCServer.Stop()
	}
	c.JSON(200, gin.H{
		"message": "successfully stop fabric",
	})
}
