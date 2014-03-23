package main

import (
	"bufio"
	"fmt"
	//"github.com/beevik/etree"
	"github.com/mattn/go-xmpp"
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

func init() {

}

func handle_chat(v *xmpp.Chat) *xmpp.Chat {
	if v.Text == "" {
		return nil
	}

	tokens := strings.SplitN(v.Text, " ", -1)
	cmd := tokens[0]

	if cmd == "!ping" {
		return &xmpp.Chat{Type: "chat", Remote: v.Remote, Text: "!pong " + v.Text[len("!ping"):]}
	} else if cmd == "!help" {
		return &xmpp.Chat{Type: "chat", Remote: v.Remote, Text: `
Available commands are:
!help            - This command
!ping <data>     - Ping test
!list            - List currently monitored stocks
!me              - Display data about yourself
!stock <stock>   - Get data about a stock
!s <stock> <per> - Subscribe to variation about a stock
		`}
	} else if cmd == "!me" {
		contact := db.GetContact(v.Remote)
		return &xmpp.Chat{Type: "chat", Remote: v.Remote, Text: fmt.Sprintf("You are contact %d (%s)", contact.Id, contact.Email)}
	} else if cmd == "!stock" && len(tokens) >= 2 {
		short := tokens[1]
		stock, err := stocks.GetStock(short)
		if err == nil {
			value, _ := stock.GetValue()
			return &xmpp.Chat{Type: "chat", Remote: v.Remote, Text: fmt.Sprintf("Stock \"%s\" (%s:%s:%d) : %f", stock.Name, stock.Market, stock.Short, stock.Id, value)}
		} else {
			return &xmpp.Chat{Type: "chat", Remote: v.Remote, Text: fmt.Sprintf("Could not find stock %s: %v", short, err)}
		}
	} else if cmd == "!s" && len(tokens) >= 3 {
		short := tokens[1]

		stock, err := stocks.GetStock(short)

		if err != nil {
			return &xmpp.Chat{Type: "chat", Remote: v.Remote, Text: fmt.Sprintf("Could not find stock %s: %v", short, err)}
		}

		contact := db.GetContact(v.Remote)

		if contact == nil {
			return &xmpp.Chat{Type: "chat", Remote: v.Remote, Text: fmt.Sprintf("Could not get contact !")}
		}

		per, err := strconv.ParseFloat(tokens[2], 32)

		if err != nil {
			return &xmpp.Chat{Type: "chat", Remote: v.Remote, Text: fmt.Sprintf("Percentage parsing error", err)}
		}

		alert, err := db.SubscribeAlert(stock, contact, float32(per))
		if err != nil {
			return &xmpp.Chat{Type: "chat", Remote: v.Remote, Text: fmt.Sprintf("Could not save alert: %v", err)}
		}

		return &xmpp.Chat{Type: "chat", Remote: v.Remote, Text: fmt.Sprintf("Done: %v", alert)}

	} else {
		return &xmpp.Chat{Type: "chat", Remote: v.Remote, Text: fmt.Sprintf("What do you mean ? Type !help for help.")}
	}
}

func xmpp_handling() {
	for { // This program never stops
		var talk *xmpp.Client
		sleep := time.Second * 5
		for { // We try to connect in loops, but the time between connections grows with failures
			var err error
			log.Println("Connecting...")
			if par.notls {
				talk, err = xmpp.NewClientNoTLS(par.server, par.username, par.password, par.debug)
			} else {
				talk, err = xmpp.NewClient(par.server, par.username, par.password, par.debug)
			}
			if err != nil {
				log.Println("Err:", err)
				log.Printf("Sleeping %d seconds...", sleep/time.Second)
				time.Sleep(sleep)
				sleep += time.Second
				if sleep > time.Second*120 {
					sleep = time.Second * 5
				}
			} else {
				log.Println("Connected !")
				break
			}
		}

		for {
			chat, err := talk.Recv()
			if err != nil {
				log.Fatal(err)
			}
			switch v := chat.(type) {
			case xmpp.Chat:
				if v.Text != "" {
					log.Printf("[CHAT] %s --> \"%s\"", v.Remote, v.Text)
				}
				response := handle_chat(&v)
				if response != nil {
					talk.Send(*response)
				}
				//case xmpp.Presence:
				//	fmt.Println("==> PRESENCE:", v.From, v.Show)
			}
		}
	}
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
	defer stocks.Close()

	// We start the XMPP handling code
	go xmpp_handling()

	// We block on the console handling code
	console_handling()

	log.Println("Stopping !")
}
