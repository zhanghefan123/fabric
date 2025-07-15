package config

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

var (
	EnvLoaderInstance = &EnvLoader{
		IdToAddressMapping: make(map[uint64]string),
	}
)

type EnvLoader struct {
	EnableFrr                    string
	ContainerName                string
	FirstInterfaceName           string
	FirstInterfaceAddr           string
	WebServerListenPort          int
	EnableRoutine                bool
	EnableAdvancedMessageHandler bool
	EnablePprof                  bool
	IdToAddressMapping           map[uint64]string
	IsOrderer                    bool
	PProfOrdererListenPort       int
	PProfPeerListenPort          int
}

// LoadEnv 进行环境变量的加载
func (envLoader *EnvLoader) LoadEnv(isOrderer bool) error {
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
	if err != nil {
		return fmt.Errorf("parse ENABLE_ROUTINE error %v", err)
	}
	envLoader.EnableAdvancedMessageHandler, err = strconv.ParseBool(os.Getenv("ENABLE_ADVANCED_MESSAGE_HANDLER"))
	if err != nil {
		return fmt.Errorf("parse ENABLE_ADVANCED_MESSAGE_HANDLER error %v", err)
	}
	envLoader.EnablePprof, err = strconv.ParseBool(os.Getenv("ENABLE_PPROF"))
	if err != nil {
		return fmt.Errorf("parse ENABLE_PPROF error %v", err)
	}
	err = envLoader.ReadIdToAddressMapping()
	if err != nil {
		return fmt.Errorf("read id to address mapping failed %v", err)
	}
	if isOrderer {
		envLoader.IsOrderer = true
		var pprofOrdererListenPort int64
		pprofOrdererListenPort, err = strconv.ParseInt(os.Getenv("PPROF_ORDERER_LISTEN_PORT"), 10, 64)
		if err != nil {
			return fmt.Errorf("parse PPROF_ORDERER_LISTEN_PORT error: %v", err)
		}
		envLoader.PProfOrdererListenPort = int(pprofOrdererListenPort)
	} else {
		envLoader.IsOrderer = false
		var pprofPeerListenPort int64
		pprofPeerListenPort, err = strconv.ParseInt(os.Getenv("PPROF_PEER_LISTEN_PORT"), 10, 64)
		if err != nil {
			return fmt.Errorf("parse PPROF_ORDERER_LISTEN_PORT error: %v", err)
		}
		envLoader.PProfPeerListenPort = int(pprofPeerListenPort)
	}

	return nil
}

func (envLoader *EnvLoader) ReadIdToAddressMapping() error {
	filePath := fmt.Sprintf("/configuration/%s/fabric/fabric_id_to_address.conf", EnvLoaderInstance.ContainerName)
	file, err := os.Open(filePath)
	defer func() {
		errClose := file.Close()
		if err == nil {
			err = errClose
		}
	}()
	if err != nil {
		return fmt.Errorf("open file failed: %w", err)
	}
	bytesContent, err := io.ReadAll(file)

	mappings := strings.Split(string(bytesContent), "\n")
	for _, mapping := range mappings {
		var nodeId uint64
		result := strings.Split(mapping, ",")
		nodeId, err = strconv.ParseUint(result[0], 10, 64)
		if err != nil {
			return fmt.Errorf("parse index to uint failed")
		}
		address := result[1]
		envLoader.IdToAddressMapping[nodeId] = address
	}
	return nil
}
