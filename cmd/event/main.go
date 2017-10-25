package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/lodastack/event/cluster"
	"github.com/lodastack/event/config"
	"github.com/lodastack/event/loda"
	"github.com/lodastack/event/query"
	"github.com/lodastack/event/work"

	"github.com/lodastack/log"
)

func initLog(conf config.LogConfig) {
	if !conf.Enable {
		fmt.Println("log to std err")
		log.LogToStderr()
		return
	}

	if backend, err := log.NewFileBackend(conf.Path); err != nil {
		fmt.Fprintf(os.Stderr, "init logs folder failed: %s\n", err.Error())
		os.Exit(1)
	} else {
		log.SetLogging(conf.Level, backend)
		backend.Rotate(conf.FileNum, uint64(1024*1024*conf.FileSize))
	}
}

func init() {
	configFile := flag.String("c", "./conf/event.conf", "config file path")
	flag.Parse()
	fmt.Printf("load config from %s\n", *configFile)
	err := config.LoadConfig(*configFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read config file failed:\n%s\n", err.Error())
		os.Exit(1)
	}
	initLog(config.GetConfig().Log)
}

func main() {
	fmt.Printf("build via golang version: %s\n", runtime.Version())
	c, err := cluster.NewCluster(config.GetConfig().Etcd.Endpoints,
		config.GetConfig().Etcd.Auth, config.GetConfig().Etcd.Username, config.GetConfig().Etcd.Password, 5, 5)
	if err != nil {
		fmt.Printf("NewCluster error: %s\n", err.Error())
		return
	}
	go loda.UpdateAlarmsFromLoda()
	w := work.NewWork(c)
	time.Sleep(500 * time.Millisecond) // TODO
	go w.CheckAlarmLoop()
	go loda.UpdateOffMachineLoop()
	go query.Start(w)

	select {}
}
