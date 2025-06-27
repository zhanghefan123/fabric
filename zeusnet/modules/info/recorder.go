package info

import (
	"fmt"
	"github.com/hyperledger/fabric/zeusnet/variables"
	"os"
)

func WriteBlockHeight(height uint64) (err error) {
	// 文件 handle 和 错误
	var file *os.File
	filePath := fmt.Sprintf("/configuration/%s/block_height.stat", variables.EnvLoaderInstance.ContainerName)
	// 为文件 handle 赋值
	file, err = os.OpenFile(filePath, os.O_RDWR|os.O_CREATE, 0666)
	defer func() {
		if err == nil {
			err = file.Close()
		}
	}()
	// 如果出现错误就进行返回
	if err != nil {
		fmt.Printf("open block_height.stat err: %v", err)
		return
	}
	// 只进行自己的结果的写入
	result := fmt.Sprintf("%d", height) // BFTSynchronizer.support.Height
	_, err = file.WriteString(result)
	if err != nil {
		fmt.Printf("write peers_height.stat err: %v", err)
	}
	return nil
}
