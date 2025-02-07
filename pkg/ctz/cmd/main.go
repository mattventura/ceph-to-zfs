package main

import (
	"ceph-to-zfs/pkg/ctz/backup"
	"ceph-to-zfs/pkg/ctz/config"
	"ceph-to-zfs/pkg/ctz/web"
	"flag"
	"fmt"
	"os"
)

func main() {
	webEnable := flag.Bool("web", false, "enable web interface")
	webPort := flag.Int("webport", 9999, "web interface port")
	oneShot := flag.Bool("oneshot", false, "run all jobs once and exit")
	configFile := flag.String("config", "./config.yaml", "config file")
	configOnly := flag.Bool("check-config", false, "validate config file and exit")

	//webIntfFlag := flag.Uint("web-port", -1, "enable web interface")
	flag.Parse()
	cfg, err := config.FromYamlFile(*configFile)
	if err != nil {
		fmt.Printf("Error reading config file: %v\n", err)
		os.Exit(1)
	}
	if *configOnly {
		fmt.Println("Config file looks valid")
		os.Exit(0)
	}
	task := backup.NewTopLevelTask(cfg)
	if *webEnable {
		err := web.StartWebInterface(task, *webPort)
		if err != nil {
			fmt.Printf("Unable to start web interface: %v\n", err)
			os.Exit(3)
		}
	} else {
		if *webPort != 0 {
			fmt.Println("Web interface port specified, but web interface not enabled")
		}
	}
	if *oneShot {
		err = task.Run()
		if err != nil {
			fmt.Printf("Task failed: %v\n", err)
			os.Exit(1)
		} else {
			if task.StatusLog().Status().Type().IsBad() {
				os.Exit(1)
			} else {
				fmt.Println("Task completed successfully")
				os.Exit(0)
			}
		}
	} else if !*webEnable {
		fmt.Println("Neither oneshot mode nor web interface enabled, nothing to do")
		os.Exit(64)
	}
	if *webEnable && !*oneShot {
		// Block forever
		select {}
	}
}
