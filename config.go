package main

import (
	"code.google.com/p/gcfg"
	"flag"
	"fmt"
	"log"
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

func NewConfig() *Config {
	config := &Config{}
	config.Db.File = "followthestock.db"
	config.Xmpp.Username = ""
	config.Xmpp.Server = "talk.google.com:443"
	config.Xmpp.LinesPerMessage = 15
	config.Xmpp.ActivityWatchdogMinutes = 30

	var fileName string
	var showConfig bool
	flag.StringVar(&fileName, "config", "/etc/followthestock/followthestock.conf", "Config file")
	flag.BoolVar(&showConfig, "show-config", false, "Show config")

	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: followthestock -config <file>")
		flag.PrintDefaults()
		os.Exit(2)
	}
	flag.Parse()
	if err := gcfg.ReadFileInto(config, fileName); err != nil {
		log.Fatalf("Could not read config: %v", err)
	}

	if showConfig {
		log.Printf("Config: %#v", config)
	}

	return config
}
