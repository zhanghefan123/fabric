package apis

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/hyperledger/fabric/zeusnet/bft_related"
	"github.com/hyperledger/fabric/zeusnet/modules/system"
)

// StopFabric 停止 Orderer 节点
func StopFabric(c *gin.Context) {
	err := system.ParameterInstance.OpsSystem.Stop()
	if err != nil {
		c.JSON(500, gin.H{
			"message": "stop fabric error: " + err.Error(),
		})
		fmt.Println("stop fabric error: ", err.Error())
		return
	}
	err = system.ParameterInstance.AdminServer.Stop()
	if err != nil {
		c.JSON(500, gin.H{
			"message": "stop fabric admin server error: " + err.Error(),
		})
		fmt.Println("stop fabric admin server error: ", err.Error())
		return
	}
	system.ParameterInstance.GrpcServer.Stop()
	if system.ParameterInstance.ClusterGRPCServer != nil {
		system.ParameterInstance.ClusterGRPCServer.Stop()
	}
	c.JSON(200, gin.H{
		"message": "successfully stop fabric",
	})
}

// StartAttack 开启攻击
func StartAttack(c *gin.Context) {
	err := bft_related.ConsensusController.StartAttackLeader()
	if err != nil {
		fmt.Printf("error %v", err)
		c.JSON(500, gin.H{
			"message": fmt.Errorf("attack failed: %v", err),
		})
	} else {
		c.JSON(200, gin.H{
			"message": "successfully launch insider attack",
		})
	}
}

// StopAttack 停止攻击
func StopAttack(c *gin.Context) {
	err := bft_related.ConsensusController.StopAttackLeader()
	if err != nil {
		c.JSON(500, gin.H{
			"message": fmt.Errorf("stop attack failed: %v", err),
		})
	} else {
		c.JSON(200, gin.H{
			"message": "successfully stop insider attack",
		})
	}
}

// StartMaliciousSynchronize 开始向主节点进行恶意同步
func StartMaliciousSynchronize(c *gin.Context) {
	err := bft_related.ConsensusController.StartMaliciousSynchronize()
	if err != nil {
		c.JSON(500, gin.H{
			"message": fmt.Errorf("start malicious synchronize failed: %v", err),
		})
	} else {
		c.JSON(200, gin.H{
			"message": "start malicious synchronize successful",
		})
	}
}

// StopMaliciousSynchronize 停止向主节点进行恶意同步
func StopMaliciousSynchronize(c *gin.Context) {
	fmt.Println("stop malicious synchronize")
	err := bft_related.ConsensusController.StopMaliciousSynchronize()
	if err != nil {
		c.JSON(500, gin.H{
			"message": fmt.Errorf("stop malicious synchronize failed: %v", err),
		})
	} else {
		c.JSON(200, gin.H{
			"message": "start malicious synchronize successful",
		})
	}
}
