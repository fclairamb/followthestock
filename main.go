package main

import (
	"bufio"
	"fmt"
	//"github.com/beevik/etree"
	"github.com/mattn/go-xmpp"
	"log"
	"os"
	"strings"
	"time"
)

func init() {

}

func handle_chat(v *xmpp.Chat) *xmpp.Chat {
	if v.Text == "" {
		return nil
	}

	tokens := strings.SplitN(v.Text, " ", 2)
	cmd := tokens[0]

	if cmd == "!ping" {
		return &xmpp.Chat{Type: "chat", Remote: v.Remote, Text: "!pong " + v.Text[len("!ping"):]}
	} else if cmd == "!help" {
		return &xmpp.Chat{Type: "chat", Remote: v.Remote, Text: `
Available commands are:
!help           - This command
!ping <data>    - Ping test
!list           - List currently monitored stocks
!me             - Display data about yourself
		`}
	} else if cmd == "!me" {
		contact := db.GetContact(v.Remote)
		return &xmpp.Chat{Type: "chat", Remote: v.Remote, Text: fmt.Sprintf("You are contact %d (%s)", contact.Id, contact.Email)}
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

var par Parameters

func main() {

	log.Println("Starting !")

	// We parse the parameters
	ParametersParse(&par)

	// We open the database
	db = FtsDBOpen()
	defer db.Close()

	// We start the XMPP handling code
	go xmpp_handling()

	// We block on the console handling code
	console_handling()

	log.Println("Stopping !")
}
