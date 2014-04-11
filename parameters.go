package main

import (
	"flag"
	"fmt"
	"os"
)

type Parameters struct {
	dbfile            string
	server            string
	username          string
	password          string
	notls             bool
	debug             bool
	exact             bool
	nbLinesPerMessage int
}

func ParametersParse(par *Parameters) {
	flag.StringVar(&par.dbfile, "dbfile", "followthestock.db", "database file")
	flag.StringVar(&par.server, "server", "talk.google.com:443", "XMPP Server")
	flag.StringVar(&par.username, "username", "followthestock@gmail.com", "XMPP Username")
	flag.StringVar(&par.password, "password", "", "XMPP Password")
	flag.BoolVar(&par.notls, "notls", false, "Disable TLS")
	flag.BoolVar(&par.debug, "debug", false, "Enable debugging")
	flag.BoolVar(&par.exact, "exact", false, "Exact timing")
	flag.IntVar(&par.nbLinesPerMessage, "nb", 15, "Number of lines per message")

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
