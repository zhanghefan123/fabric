package frr

import (
	"fmt"
	"github.com/hyperledger/fabric/zeusnet/modules/config"
	"github.com/hyperledger/fabric/zeusnet/tools/execute"
)

// StartFrr 进行 frr 的启动
func StartFrr() error {
	if config.EnvLoaderInstance.EnableFrr == "true" {
		fmt.Println("start frr")
		err := CopyFrrConfigurationFile()
		if err != nil {
			return fmt.Errorf("failed to copy frr configuration file %w", err)
		}
		err = execute.Command("service", []string{"frr", "start"})
		if err != nil {
			return fmt.Errorf("failed to start frr service: %w", err)
		}
	} else {
		fmt.Println("not start frr")
	}
	return nil
}

// CopyFrrConfigurationFile 进行 frr 配置文件的拷贝
func CopyFrrConfigurationFile() error {
	sourceFilePath := fmt.Sprintf("/configuration/%s/route/frr.conf", config.EnvLoaderInstance.ContainerName)
	targetFilePath := "/etc/frr/frr.conf"
	err := execute.Command("cp", []string{sourceFilePath, targetFilePath})
	if err != nil {
		return fmt.Errorf("failed to execute cp command: %w", err)
	}
	return nil
}
