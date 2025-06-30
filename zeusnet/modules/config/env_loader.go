package config

import (
	"fmt"
	"os"
	"strconv"
)

type EnvLoader struct {
	EnableFrr                    string
	ContainerName                string
	FirstInterfaceName           string
	FirstInterfaceAddr           string
	WebServerListenPort          int
	EnableRoutine                bool
	EnableAdvancedMessageHandler bool
}

// LoadEnv 进行环境变量的加载
func (envLoader *EnvLoader) LoadEnv() error {
	envLoader.EnableFrr = os.Getenv("ENABLE_FRR")
	envLoader.ContainerName = os.Getenv("CONTAINER_NAME")
	envLoader.FirstInterfaceName = os.Getenv("FIRST_INTERFACE_NAME")
	envLoader.FirstInterfaceAddr = os.Getenv("FIRST_INTERFACE_ADDR")
	webServerListenPort, err := strconv.ParseInt(os.Getenv("WEB_SERVER_LISTEN_PORT"), 10, 64)
	if err != nil {
		return fmt.Errorf("parse WEB_SERVER_LISTEN_PORT error: %v", err)
	}
	envLoader.WebServerListenPort = int(webServerListenPort)
	envLoader.EnableRoutine, err = strconv.ParseBool(os.Getenv("ENABLE_ROUTINE"))
	envLoader.EnableAdvancedMessageHandler, err = strconv.ParseBool(os.Getenv("ENABLE_ADVANCED_MESSAGE_HANDLER"))
	if err != nil {
		return fmt.Errorf("parse ENABLE_ROUTINE error %v", err)
	}
	return nil
}
