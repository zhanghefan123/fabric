# 1. 修改的文件

- fabric-3.0.0/internal/peer/node/start.go 添加了启动运行 frr 的过程
- fabric-3.0.0/cmd/orderer/main.go 添加了启动运行 frr 的过程
- 上述两个启动项共同使用 fabric-3.0.0/zeusnet/starter.go 之中的启动程序