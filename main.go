package main

import (
	"bufio"
	"fmt"
	//"github.com/beevik/etree"
	"log"
	"os"

	"strings"
)

func init() {

}

func console_handling() {
	in := bufio.NewReader(os.Stdin)
	for {
		fmt.Printf("> ")
		line, err := in.ReadString('\n')
		if err != nil {
			continue
		}
		line = strings.TrimRight(line, "\n")

		tokens := strings.SplitN(line, " ", 2)
		if tokens[0] == "" {
			continue
		} else if tokens[0] == "quit" {
			break
		} else {
			fmt.Printf("\"%s\" not understood !", tokens[0])
		}
	}
}

var xm *FtsXmpp
var db *FtsDB
var stocks *StocksMgmt

var par Parameters

func main() {

	log.Println("Starting !")

	// We parse the parameters
	ParametersParse(&par)

	// We open the database
	db = NewFtsDB(&par)
	defer db.Close()

	// We load the stocks
	stocks = NewStocksMgmt()
	stocks.Start()
	defer stocks.Stop()

	// We start the XMPP handling code
	xm = NewFtsXmpp(&par)
	xm.Start()
	defer xm.Stop()

	// We block on the console handling code
	console_handling()

	log.Println("Stopping !")
}
