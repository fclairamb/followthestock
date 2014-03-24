package main

import (
	"errors"
	"fmt"
	"github.com/mattn/go-xmpp"
	"log"
	"strconv"
	"strings"
	"time"
)

type FtsXmpp struct {
	par  *Parameters
	clt  *xmpp.Client
	Recv chan interface{}
	Send chan interface{}
}

type SendChat struct {
	Remote, Text string
}

func NewFtsXmpp(par *Parameters) *FtsXmpp {
	return &FtsXmpp{par: par, Recv: make(chan interface{}, 10), Send: make(chan interface{}, 10)}
}

func (x *FtsXmpp) handle_chat(v *xmpp.Chat) (err error) {
	if v.Text == "" {
		return nil
	}

	tokens := strings.SplitN(v.Text, " ", -1)
	cmd := tokens[0]

	if cmd == "!ping" {
		x.Send <- &SendChat{Remote: v.Remote, Text: "!pong " + v.Text[len("!ping"):]}
	} else if cmd == "!help" {
		x.Send <- &SendChat{Remote: v.Remote, Text: `
Available commands are:

!help             - This command
!s <stock> <per>  - Subscribe to variation about a stock
!u <stock>        - Unsubscribe from a stock
!list             - List currently monitored stocks
!ping <data>      - Ping test
!me               - Display data about yourself
!stock <stock>    - Get data about a stock
`}
	} else if cmd == "!me" {
		contact := db.GetContactFromEmail(v.Remote)
		x.Send <- &SendChat{Remote: v.Remote, Text: fmt.Sprintf("You are contact %d (%s)", contact.Id, contact.Email)}
	} else if cmd == "!stock" && len(tokens) >= 2 {
		short := tokens[1]
		stock, err := stocks.GetStock(short)
		if err == nil {
			value, _ := stock.GetValue()
			x.Send <- &SendChat{Remote: v.Remote, Text: fmt.Sprintf("Stock \"%s\" (%s:%s:%d) : %f", stock.Name, stock.Market, stock.Short, stock.Id, value)}
		} else {
			x.Send <- &SendChat{Remote: v.Remote, Text: fmt.Sprintf("Could not find stock %s: %v", short, err)}
		}
	} else if cmd == "!s" && len(tokens) == 3 {
		short := tokens[1]

		stock, err := stocks.GetStock(short)

		if err != nil {
			return err
		}

		contact := db.GetContactFromEmail(v.Remote)

		if contact == nil {
			return errors.New("Could not get contact !")
		}

		per, err := strconv.ParseFloat(tokens[2], 32)

		if err != nil {
			return err
		}

		alert, err := stocks.SubscribeAlert(stock, contact, float32(per))
		if err != nil {
			return err
		}

		x.Send <- &SendChat{Remote: v.Remote, Text: fmt.Sprintf("Done: %v", alert)}

	} else if cmd == "!u" && len(tokens) == 2 {
		short := tokens[1]

		stock, err := stocks.GetStock(short)

		if err != nil {
			return err
		}

		contact := db.GetContactFromEmail(v.Remote)

		if contact == nil {
			return errors.New("Could not get contact !")
		}

		err = stocks.UnsubscribeAlert(stock, contact)

		if err != nil {
			return err
		}

		x.Send <- &SendChat{Remote: v.Remote, Text: fmt.Sprintf("Done !")}

	} else if cmd == "!list" {
		c := db.GetContactFromEmail(v.Remote)

		if c == nil {
			return errors.New("Could not get contact !")
		}
		i := 0
		msg := ""
		//log.Println("Contact", c)
		for _, al := range *db.GetAlertsForContact(c) {
			//log.Println(al)
			i++
			s := db.GetStockFromId(al.Stock)
			if s == nil {
				db.DeleteAlert(&al)
				continue
			}
			msg += fmt.Sprintf("\n%s - %f%% / %d", s.String(), al.Percent, al.Id)

			if i%5 == 0 {
				x.Send <- &SendChat{Remote: v.Remote, Text: msg}
				msg = ""
			}
		}
		if i == 0 {
			x.Send <- &SendChat{Remote: v.Remote, Text: "You didn't subscribe to anything !"}
		}
		if len(msg) != 0 {
			x.Send <- &SendChat{Remote: v.Remote, Text: msg}
			msg = ""
		}
	} else if cmd == "!forgetme" {
		contact := db.GetContactFromEmail(v.Remote)

		if contact == nil {
			return errors.New("Could not get contact !")
		}

		db.DeleteContact(contact)
	} else {
		x.Send <- &SendChat{Remote: v.Remote, Text: fmt.Sprintf("What do you mean ? Type !help for help.")}
	}

	return nil
}

func (x *FtsXmpp) runRecv() {
	for {
		msg := <-x.Recv
		//log.Println("Recv:", msg)
		switch v := msg.(type) {
		case xmpp.Chat:
			if v.Text != "" {
				log.Printf("[CHAT] %s --> \"%s\"", v.Remote, v.Text)
			}
			err := x.handle_chat(&v)
			if err != nil {
				x.Send <- &SendChat{Remote: v.Remote, Text: fmt.Sprintln("Processing your message created an error:", err)}
			}
		}
	}
}

func (x *FtsXmpp) runSend() {
	for {
		msg := <-x.Send
		//log.Println("Send:", msg)
		switch v := msg.(type) {
		case *SendChat:
			log.Printf("[CHAT] %s <-- \"%s\"", v.Remote, v.Text)
			x.clt.Send(xmpp.Chat{Type: "chat", Remote: v.Remote, Text: v.Text})
		}
	}
}

func (x *FtsXmpp) runMain() {
	for { // This program never stops
		sleep := time.Second * 5
		for { // We try to connect in loops, but the time between connections grows with failures
			var err error
			log.Println("Connecting...")
			if par.notls {
				x.clt, err = xmpp.NewClientNoTLS(x.par.server, x.par.username, x.par.password, x.par.debug)
			} else {
				x.clt, err = xmpp.NewClient(x.par.server, x.par.username, x.par.password, x.par.debug)
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
			msg, err := x.clt.Recv()
			if err != nil {
				log.Println(err)
				break
			}
			x.Recv <- msg
		}
	}
}

func (x *FtsXmpp) Start() {
	go x.runMain()
	go x.runRecv()
	go x.runSend()
}

func (x *FtsXmpp) Stop() {

}
