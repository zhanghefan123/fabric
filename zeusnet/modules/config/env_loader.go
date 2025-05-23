package config

import (
	"os"
)

type EnvLoader struct {
	EnableFrr          string
	ContainerName      string
	FirstInterfaceName string
	FirstInterfaceAddr string
}

// LoadEnv 进行环境变量的加载
func (envLoader *EnvLoader) LoadEnv() {
	envLoader.EnableFrr = os.Getenv("ENABLE_FRR")
	envLoader.ContainerName = os.Getenv("CONTAINER_NAME")
	envLoader.FirstInterfaceName = os.Getenv("FIRST_INTERFACE_NAME")
	envLoader.FirstInterfaceAddr = os.Getenv("FIRST_INTERFACE_ADDR")
}
