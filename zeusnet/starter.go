//go:build linux

package zeusnet

import (
	"fmt"
	"github.com/hyperledger/fabric/zeusnet/modules/config"
	"github.com/hyperledger/fabric/zeusnet/modules/frr"
	"github.com/hyperledger/fabric/zeusnet/tools/network"
	"log"
	"net/http"
	"os"
	"runtime"
)

func Start(isOrderer bool) error {
	// 1. 进行配置文件的加载
	err := config.EnvLoaderInstance.LoadEnv(isOrderer)
	if err != nil {
		return fmt.Errorf("load env error: %v", err)
	}
	fmt.Println(config.EnvLoaderInstance)
	// 3. 等待接口启动
	err = network.SleepUntilInterfaceUp(config.EnvLoaderInstance.FirstInterfaceName, config.EnvLoaderInstance.FirstInterfaceAddr)
	if err != nil {
		return fmt.Errorf("return until interface up error: %v", err)
	}
	// 4. frr 的启动程序
	err = frr.StartFrr()
	if err != nil {
		return fmt.Errorf("start frr failed: %w", err)
	}
	// 5. 进行性能监视函数的启动
	// -----------------------------------------
	if config.EnvLoaderInstance.EnablePprof {
		log.SetFlags(log.Lshortfile | log.LstdFlags)
		log.SetOutput(os.Stdout)
		runtime.SetMutexProfileFraction(1)
		runtime.SetBlockProfileRate(1)
		go func() {
			if isOrderer {
				if err = http.ListenAndServe(fmt.Sprintf(":%d", config.EnvLoaderInstance.PProfOrdererListenPort), nil); err != nil {
					log.Fatal(err)
				}
			} else {
				if err = http.ListenAndServe(fmt.Sprintf(":%d", config.EnvLoaderInstance.PProfPeerListenPort), nil); err != nil {
					log.Fatal(err)
				}
			}
		}()
	} else {
		fmt.Println("not start pprof")
	}
	// -----------------------------------------

	return nil
}
