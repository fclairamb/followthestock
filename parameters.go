package main

import (
	"flag"
	"fmt"
	"os"
)

type Parameters struct {
	server   string
	username string
	password string
	notls    bool
	debug    bool
}

func ParametersParse(par *Parameters) {
	flag.StringVar(&par.server, "server", "talk.google.com:443", "server")
	flag.StringVar(&par.username, "username", "followthestock@gmail.com", "username")
	flag.StringVar(&par.password, "password", "SuperStock", "password")
	flag.BoolVar(&par.notls, "notls", false, "Disable TLS")
	flag.BoolVar(&par.debug, "debug", false, "Enable debugging")

	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: followthestock -username toto@gmail.com -password pass")
		flag.PrintDefaults()
		os.Exit(2)
	}
	flag.Parse()
	if par.username == "" || par.password == "" {
		flag.Usage()
	}
}
