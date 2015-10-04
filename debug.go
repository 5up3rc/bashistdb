// +build debug

package main

import (
	"fmt"
	"net/http"
	_ "net/http/pprof"

	conf "github.com/andmarios/bashistdb/configuration"
)

func init() {
	Version += "-pprof"
	// Set up debug server
	go func() {
		fmt.Println(http.ListenAndServe("localhost:6060", nil))
	}()
	conf.Log.Info.Print("pprof is running at localhost:6060")
}
