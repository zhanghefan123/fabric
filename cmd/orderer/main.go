/*
Copyright IBM Corp. 2017 All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

// Package main is the entrypoint for the orderer binary
// and calls only into the server.Main() function.  No other
// function should be included in this package.
package main

import (
	"fmt"
	"github.com/hyperledger/fabric/orderer/common/server"
	"github.com/hyperledger/fabric/zeusnet"
)

func main() {
	// zeusnet add code
	// --------------------------------------------------------
	err := zeusnet.Start(true)
	if err != nil {
		fmt.Printf("error start frr %v", err)
		return
	} else {
		fmt.Println("frr started")
	}
	// --------------------------------------------------------

	// 主函数
	server.Main()
}
