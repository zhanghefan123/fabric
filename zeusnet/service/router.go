package service

import (
	"github.com/gin-gonic/gin"
	"github.com/hyperledger/fabric/zeusnet/service/apis"
)

var postRoutes = map[string]gin.HandlerFunc{
	"/stop":                        apis.StopFabric,                // 停止 Fabric 网络
	"/start_attack":                apis.StartAttack,               // 向 leader 发起攻击, 进行消息的重放
	"/stop_attack":                 apis.StopAttack,                // 停止向 leader 发起攻击
	"/start_malicious_synchronize": apis.StartMaliciousSynchronize, // 向主节点进行恶意的区块同步
}

// CORSMiddleware 中间件处理跨域问题
func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Origin, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

// InitRouter 进行 gin 引擎的创建
func InitRouter() *gin.Engine {
	r := gin.New()
	r.Use(gin.Logger())
	r.Use(gin.Recovery())
	r.Use(CORSMiddleware())
	for route, callback := range postRoutes {
		r.POST(route, callback)
	}
	return r
}
