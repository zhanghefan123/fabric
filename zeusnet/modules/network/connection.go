package network

import (
	"fmt"
	"github.com/hyperledger/fabric/zeusnet/modules/config"
	"github.com/hyperledger/fabric/zeusnet/tools/execute"
	"strings"
)

func GetConnectedTcpConnectionCount() (int, error) {
	result, err := execute.CommandWithResult("netstat", []string{"-antp"})
	if err != nil {
		return 0, fmt.Errorf("execute netstat -antp | grep ESTABLISHED error: %v", err)
	} else {
		count := 0
		lines := strings.Split(result, "\n")
		//fmt.Println("----------------------------------")
		for _, line := range lines {
			if strings.Contains(line, config.EnvLoaderInstance.FirstInterfaceAddr) && strings.Contains(line, "tcp6") && strings.Contains(line, "ESTABLISHED") {
				//fmt.Println(line)
				count += 1
			}
		}
		//fmt.Println("----------------------------------")
		return count, nil
	}
}

func GetHalfConnectedTcpConnectionCount() (int, error) {
	result, err := execute.CommandWithResult("netstat", []string{"-antp"})
	if err != nil {
		return 0, fmt.Errorf("execute netstat -antp | grep SYN_RECV error: %v", err)
	} else {
		count := 0
		lines := strings.Split(result, "\n")
		//fmt.Println("----------------------------------")
		for _, line := range lines {
			if strings.Contains(line, config.EnvLoaderInstance.FirstInterfaceAddr) && strings.Contains(line, "SYN_RECV") {
				//fmt.Println("yes", line)
				count += 1
			} else {
				//fmt.Println("not", line)
			}
		}
		//fmt.Println("----------------------------------")
		return count, nil
	}
}
