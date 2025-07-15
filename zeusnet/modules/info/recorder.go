package info

import (
	"fmt"
	"github.com/hyperledger/fabric/zeusnet/modules/config"
	"io/ioutil"
	"os"
)

type Information struct {
	BlockHeight          int
	ConnectedTcpCount    int
	HalfConnetedTcpCount int
	TimeoutCount         int
	BusMessageCount      int
}

func WriteInformation(information *Information) error {
	// 路径
	filePath := fmt.Sprintf("/configuration/%s/information.stat", config.EnvLoaderInstance.ContainerName)
	// 进行结果的写入
	result := fmt.Sprintf("%d,%d,%d,%d,%d",
		information.BlockHeight,
		information.ConnectedTcpCount,
		information.HalfConnetedTcpCount,
		information.TimeoutCount,
		information.BusMessageCount)
	// 构建临时文件
	tmpFilename := filePath + ".tmp"
	err := ioutil.WriteFile(tmpFilename, []byte(result), 0644)
	if err != nil {
		return fmt.Errorf("write tmp file error: %v", err)
	}
	// 进行文件重命名
	err = os.Rename(tmpFilename, filePath)
	if err != nil {
		return fmt.Errorf("rename tmp file error: %v", err)
	}
	return nil
}
