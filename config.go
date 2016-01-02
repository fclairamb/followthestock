package main

import (
	"code.google.com/p/gcfg"
	"flag"
	"fmt"
	"os"
)

type Config struct {
	Xmpp struct {
		Server                  string
		Username                string
		Password                string
		Notls                   bool
		Debug                   bool
		LinesPerMessage         int
		ActivityWatchdogMinutes int
	}

	General struct {
		ExactTiming bool
	}

	Db struct {
		File string
	}
}

var Console bool

var config Config

func init() {
	config.Db.File = "followthestock.db"
	config.Xmpp.Username = ""
	config.Xmpp.Server = "talk.google.com:443"
	config.Xmpp.LinesPerMessage = 15
	config.Xmpp.ActivityWatchdogMinutes = 30

	var fileName string
	var showConfig bool
	flag.StringVar(&fileName, "config", "/etc/followthestock/followthestock.conf", "Config file")
	flag.BoolVar(&showConfig, "show-config", false, "Show config")
	flag.BoolVar(&Console, "console", false, "Use console")

	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: followthestock -config <file>")
		flag.PrintDefaults()
		os.Exit(2)
	}
	flag.Parse()
	if err := gcfg.ReadFileInto(&config, fileName); err != nil {
		fmt.Fprintln(os.Stderr, "Could not read config: ", fileName)
	}

	if showConfig {
		fmt.Printf("Config: %#v\n", config)
	}
}
