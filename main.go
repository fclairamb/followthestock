package main

import (
	"bufio"
	"fmt"
	//"github.com/beevik/etree"
	"log"
	"os"

	"strings"
)

var xm *FtsXmpp
var db *FtsDB
var stocks *StocksMgmt

var par Parameters

var waitForRc chan int

func init() {
	waitForRc = make(chan int)
}

func console_handling() {
	in := bufio.NewReader(os.Stdin)
	for {
		fmt.Printf("> ")
		line, err := in.ReadString('\n')
		if err != nil {
			log.Fatal(err)
			continue
		}
		line = strings.TrimRight(line, "\n")

		tokens := strings.SplitN(line, " ", 2)
		if tokens[0] == "" {
			continue
		} else if tokens[0] == "quit" {
			waitForRc <- 0
		} else {
			fmt.Printf("\"%s\" not understood !", tokens[0])
		}
	}
}

func core() (rc int) {
	// We open the database
	db = NewFtsDB(&par)
	defer db.Close()

	// We load the stocks
	stocks = NewStocksMgmt(&par)
	stocks.Start()
	defer stocks.Stop()

	// We start the XMPP handling code
	xm = NewFtsXmpp(&par)
	xm.Start()
	defer xm.Stop()

	// We block on the console handling code
	go console_handling()

	// We wait for someone to trigger the result code
	rc = <-waitForRc

	log.Println("Stopping !")

	return
}

func main() {

	log.Println("Starting !")

	// We parse the parameters
	ParametersParse(&par)

	rc := core()

	log.Println("Bye !")

	os.Exit(rc)
}
