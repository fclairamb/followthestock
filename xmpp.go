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
	clt       *xmpp.Client
	Recv      chan interface{}
	Send      chan interface{}
	StartTime time.Time
}

type SendChat struct {
	Remote, Text string
}

func NewFtsXmpp() *FtsXmpp {
	return &FtsXmpp{
		Recv:      make(chan interface{}, 10),
		Send:      make(chan interface{}, 10),
		StartTime: time.Now().UTC(),
	}
}

func (x *FtsXmpp) handle_chat(v *xmpp.Chat) (err error) {
	if v.Text == "" {
		return nil
	}

	/* No XOR
	if strings.HasPrefix(v.Text, "test ") ^ x.par.test {
		log.Println("Wrong mode (test or no test)")
		return
	}
	*/

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
!g <stock>        - Get data about a stock
!ls               - List currently monitored stocks
!pause <days>     - Pause alerts for X days
!resume           - Resume alerts
!uptime           - Server uptime
!ping <data>      - Ping test
!me               - Display data about yourself
`}
	} else if cmd == "!me" {
		contact := db.GetContactFromEmail(v.Remote)
		x.Send <- &SendChat{Remote: v.Remote, Text: fmt.Sprintf("You are contact %d (%s)", contact.Id, contact.Email)}
	} else if cmd == "!g" {
		if len(tokens) != 2 {
			return errors.New("No stock provided !")
		}
		short := tokens[1]
		stock, err := stocks.GetStock(short)
		if err == nil {
			value, _ := stock.GetValue()
			x.Send <- &SendChat{Remote: v.Remote, Text: fmt.Sprintf("Stock %s : %.3f", stock, value)}
		} else {
			x.Send <- &SendChat{Remote: v.Remote, Text: fmt.Sprintf("Could not find stock %s: %v", short, err)}
		}
	} else if cmd == "!s" {
		if len(tokens) != 3 {
			return errors.New("You must precise stock and percentage !")
		}
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

		x.Send <- &SendChat{Remote: v.Remote, Text: fmt.Sprintf("Subscribed to %v with %.2f%% variation on alert %d.", stock, per, alert.Id)}

	} else if cmd == "!u" {
		if len(tokens) != 2 {
			return errors.New("No stock provided !")
		}

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

	} else if cmd == "!ls" {
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
			msg += fmt.Sprintf("\n%s - %.2f%% [%d]", s.String(), al.Percent, al.Id)

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
	} else if cmd == "!pause" {
		if len(tokens) != 2 {
			return errors.New("You have to specify a number of days !")
		}
		contact := db.GetContactFromEmail(v.Remote)

		if contact == nil {
			return errors.New("Could not get contact !")
		}

		var nb int64
		nb, err = strconv.ParseInt(tokens[1], 10, 64)

		if err != nil {
			return err
		}

		contact.PauseUntil = time.Now().UTC().UnixNano() + time.Hour.Nanoseconds()*24*nb

		db.SaveContact(contact)

		x.Send <- &SendChat{Remote: v.Remote, Text: fmt.Sprintf("OK, no alert for %d days.", nb)}
	} else if cmd == "!resume" {
		contact := db.GetContactFromEmail(v.Remote)

		if contact == nil {
			return errors.New("Could not get contact !")
		}

		contact.PauseUntil = 0

		db.SaveContact(contact)
		x.Send <- &SendChat{Remote: v.Remote, Text: "OK, back to work!"}
	} else if cmd == "!forgetme" {
		contact := db.GetContactFromEmail(v.Remote)

		if contact == nil {
			return errors.New("Could not get contact !")
		}

		x.Send <- &SendChat{Remote: v.Remote, Text: "Who are you ?"}

		db.DeleteContact(contact)
	} else if cmd == "!uptime" {
		diff := time.Now().UTC().Sub(x.StartTime)
		diff -= diff % time.Second
		x.Send <- &SendChat{Remote: v.Remote, Text: fmt.Sprintf("Uptime: %s", diff)}
	} else if cmd == "!quit" {
		x.Send <- &SendChat{Remote: v.Remote, Text: "Bye bye!"}
		time.Sleep(time.Second * 5)
		waitForRc <- 1
	} else if cmd == "!version" {
		x.Send <- &SendChat{Remote: v.Remote, Text: "version = " + FTS_VERSION}
	} else {
		if cmd == "What" {
			log.Println("Potential loophole")
			return nil
		}
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
				x.Send <- &SendChat{Remote: v.Remote, Text: fmt.Sprint("Error:", err)}
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
				x.clt, err = xmpp.NewClientNoTLS(par.server, par.username, par.password, par.debug)
			} else {
				x.clt, err = xmpp.NewClient(par.server, par.username, par.password, par.debug)
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
