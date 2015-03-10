package main

import (
	"errors"
	"fmt"
	"github.com/mattn/go-xmpp"
	"log"
	"math"
	"strconv"
	"strings"
	"time"
)

type FtsXmpp struct {
	clt          *xmpp.Client
	Recv         chan interface{}
	Send         chan interface{}
	StartTime    time.Time
	lastRcvdData time.Time
	checkTicker  *time.Ticker
}

type SendChat struct {
	Remote, Text string
}

func NewFtsXmpp() *FtsXmpp {
	return &FtsXmpp{
		Recv:         make(chan interface{}, 10),
		Send:         make(chan interface{}, 10),
		StartTime:    time.Now().UTC(),
		checkTicker:  time.NewTicker(time.Minute * 5),
		lastRcvdData: time.Now().UTC(),
	}
}

func (x *FtsXmpp) handle_chat(v *xmpp.Chat) (err error) {
	if v.Text == "" {
		return nil
	}

	v.Text = strings.ToLower(strings.TrimSpace(v.Text))

	tokens := strings.SplitN(v.Text, " ", -1)
	cmd := tokens[0]

	// We now ignore the "!" prefix
	if cmd[0] == '!' {
		cmd = cmd[1:]
	}

	switch cmd {
	case "ping":
		{
			x.Send <- &SendChat{Remote: v.Remote, Text: "!pong " + v.Text[len("!ping"):]}
		}
	case "help":
		{
			x.Send <- &SendChat{Remote: v.Remote, Text: `
Available commands are:

help                              - This command
s <stock> (+|-)<per> (<duration>) - Subscribe to variation about a stock (Ex: !s rno 2)
u <stock>                         - Unsubscribe from a stock
g <stock>                         - Get data about a stock
ls                                - List currently monitored stocks
v                                 - Get the value of our stocks
v <stock>                         - Get the value of a particular stock
v <stock> <nb> (<cost>)           - Register the number of shares and the cost of a particular stock
pause <days>                      - Pause alerts for X days
resume                            - Resume alerts
uptime                            - Application uptime
url                               - Show an URL with alerts
nourl                             - Do not show an URL with alerts
ping <data>                       - Ping test
`}
		}
	case "me":
		{
			contact := db.GetContactFromEmail(v.Remote)
			x.Send <- &SendChat{Remote: v.Remote, Text: fmt.Sprintf("You are contact %d (%s)", contact.Id, contact.Email)}
		}
	case "g":
		{
			if len(tokens) != 2 {
				return errors.New("No stock provided !")
			}
			short := tokens[1]
			stock, err := stocks.GetStock(short)
			if err == nil {
				value, _, _ := stock.GetValue()
				x.Send <- &SendChat{Remote: v.Remote, Text: fmt.Sprintf("Stock %s : %.3f %s", stock, value, stock.Currency)}
			} else {
				x.Send <- &SendChat{Remote: v.Remote, Text: fmt.Sprintf("Could not find stock \"%s\".", short)}
			}
		}
	case "s":
		{
			if len(tokens) < 3 {
				return errors.New("You must specify stock and percentage !")
			}
			short := tokens[1]

			stock, err := stocks.GetStock(short)

			if err != nil {
				return errors.New(fmt.Sprintf("Could not find the stock \"%s\".", short))
			}

			contact := db.GetContactFromEmail(v.Remote)

			if contact == nil {
				return errors.New("Could not get contact !")
			}

			value := tokens[2]

			// We remove the "%" if there's one
			value = strings.SplitN(value, "%", 2)[0]

			per, err := strconv.ParseFloat(value, 32)

			if err != nil {
				return err
			}

			per = math.Abs(per)
			var direction int
			switch value[0] {
			case '-':
				direction = ALERT_DIRECTION_DOWN
			case '+':
				direction = ALERT_DIRECTION_UP
			default:
				direction = ALERT_DIRECTION_BOTH
			}

			duration := int64(0)

			if len(tokens) >= 4 { // For duration
				if d, err := time.ParseDuration(tokens[3]); err == nil {
					duration = int64(d)
				} else {
					return err
				}
			}

			alert, err := stocks.SubscribeAlert(stock, contact, float32(per), direction, duration)
			if err != nil {
				return err
			}

			message := fmt.Sprintf("Defined alert %s", alert.String())
			x.Send <- &SendChat{Remote: v.Remote, Text: message}

		}
	case "u":
		{
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

		}
	case "l":
	case "ls":
		{
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
				msg += fmt.Sprintf("\n%s", al.String())

				if i%config.Xmpp.LinesPerMessage == 0 {
					x.Send <- &SendChat{Remote: v.Remote, Text: msg}
					msg = ""
				}
			}
			if i == 0 {
				x.Send <- &SendChat{Remote: v.Remote, Text: "You didn't subscribe to anything !"}
			}
			if msg != "" {
				x.Send <- &SendChat{Remote: v.Remote, Text: msg}
			}
		}
	case "v":
		{

			// We get the contact
			contact := db.GetContactFromEmail(v.Remote)
			if contact == nil {
				return errors.New("Could not get contact !")
			}

			if len(tokens) > 1 {
				// We get the stock
				stock, err := stocks.GetStock(tokens[1])
				if err != nil {
					return err
				}

				save := false

				csv := db.GetContactStockValue(contact.Id, stock.Id)

				if len(tokens) >= 3 {
					v, err := strconv.ParseInt(tokens[2], 10, 32)
					if err != nil {
						return err
					}
					csv.Nb = int32(v)
					if csv.Nb > 0 {
						save = true
					} else {
						db.DeleteContactStockValue(csv)
					}
				}

				if len(tokens) >= 4 {
					v, err := strconv.ParseFloat(tokens[3], 32)
					if err != nil {
						return err
					}
					csv.Value = float32(v)
				}

				if save {
					if err := db.SaveContactStockValue(csv); err != nil {
						return err
					}

					x.Send <- &SendChat{Remote: v.Remote, Text: fmt.Sprintf("Saved %s with %d x %.02f = %.02f %s [%d]", stock, csv.Nb, csv.Value, (float32(csv.Nb) * csv.Value), stock.Currency, csv.Id)}
				}
			}

			{ // We send the the message
				i := 0
				msg := ""
				totalCost := float32(0)
				totalValue := float32(0)
				for _, csv := range *db.GetContactStockValuesFromContact(contact) {
					s := db.GetStockFromId(csv.Stock)
					if s == nil {
						db.DeleteContactStockValue(&csv)
						continue
					}

					i++

					cost := float32(csv.Nb) * csv.Value
					value := float32(csv.Nb) * s.Value

					totalCost += cost
					totalValue += value

					diff := value - cost
					per := diff * 100 / cost

					msg += fmt.Sprintf(
						"\n%s, %d shares, value: %.03f / %.03f, total: %.03f - %.03f = %+.03f %s (%+.02f%%)",
						s.String(), csv.Nb, s.Value, csv.Value, value, cost, diff, s.Currency, per)

					if i%config.Xmpp.LinesPerMessage == 0 {
						x.Send <- &SendChat{Remote: v.Remote, Text: msg}
						msg = ""
					}

				}
				if i == 0 {
					x.Send <- &SendChat{Remote: v.Remote, Text: "You didn't register any stock value."}
				} else {
					totalDiff := totalValue - totalCost
					per := totalDiff * 100 / totalCost
					msg += fmt.Sprintf("\nTotal: %.03f - %.03f = %+.03f %s (%+.02f%%)", totalValue, totalCost, totalDiff, "EUR", per)
				}
				x.Send <- &SendChat{Remote: v.Remote, Text: msg}
			}

		}
	case "pause":
		{
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
		}
	case "resume":
		{
			contact := db.GetContactFromEmail(v.Remote)

			if contact == nil {
				return errors.New("Could not get contact !")
			}

			contact.PauseUntil = 0

			db.SaveContact(contact)
			x.Send <- &SendChat{Remote: v.Remote, Text: "OK, back to work !"}
		}
	case "url":
	case "nourl":
		{
			contact := db.GetContactFromEmail(v.Remote)
			if contact == nil {
				return errors.New("Could not get contact !")
			}

			contact.ShowUrl = (cmd == "!url")
			db.SaveContact(contact)
			x.Send <- &SendChat{Remote: v.Remote, Text: fmt.Sprintf("OK (ShowUrl=%v)", contact.ShowUrl)}
		}
	case "forgetme":
		{
			contact := db.GetContactFromEmail(v.Remote)

			if contact == nil {
				return errors.New("Could not get contact !")
			}

			x.Send <- &SendChat{Remote: v.Remote, Text: "Who are you ?"}

			db.DeleteContact(contact)
		}
	case "uptime":
		{
			diff := time.Now().UTC().Sub(x.StartTime)
			diff -= diff % time.Second
			x.Send <- &SendChat{Remote: v.Remote, Text: fmt.Sprintf("Uptime: %s", diff)}
		}
	case "quit":
		{
			x.Send <- &SendChat{Remote: v.Remote, Text: "Bye bye!"}
			time.Sleep(time.Second * 5)
			waitForRc <- 1
		}
	case "version":
		{
			x.Send <- &SendChat{Remote: v.Remote, Text: "version = " + FTS_VERSION}
		}
	default:
		{
			if cmd == "WHAT? " {
				log.Printf("Potential loophole: %s", v.Text)
				return nil
			}
			x.Send <- &SendChat{Remote: v.Remote, Text: fmt.Sprintf("WHAT? Type \"help\". You issued \"%s\".", v.Text)}
		}
	}

	return nil
}

func (x *FtsXmpp) runRecv() {
	for {
		msg := <-x.Recv
		x.lastRcvdData = time.Now().UTC()
		switch v := msg.(type) {
		case xmpp.Chat:
			if v.Text != "" {
				log.Printf("[CHAT] %s --> \"%s\"", v.Remote, v.Text)
			}
			err := x.handle_chat(&v)
			if err != nil {
				x.Send <- &SendChat{Remote: v.Remote, Text: fmt.Sprint("Error:", err)}
			}
		default:
			log.Printf("[XMPP] Received: %v", msg)
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
			for x.clt == nil {
				log.Printf("Cannot send on a nil XMPP !")
				time.Sleep(5)
			}
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

			xmpp.DefaultConfig.InsecureSkipVerify = true

			if config.Xmpp.Notls {
				x.clt, err = xmpp.NewClientNoTLS(config.Xmpp.Server, config.Xmpp.Username, config.Xmpp.Password, config.Xmpp.Debug)
			} else {
				x.clt, err = xmpp.NewClient(config.Xmpp.Server, config.Xmpp.Username, config.Xmpp.Password, config.Xmpp.Debug)
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
				sleep = time.Second * 2
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

func (x *FtsXmpp) runCheck() {
	// It seems the GO-XMPP library has a bug that occurs rarely. As I currently couldn't diagnose it precisely,
	// i'm adding a watchdog code
	for {
		<-x.checkTicker.C
		elapsed := time.Now().UTC().Sub(x.lastRcvdData)
		log.Printf("Last received data: %v / %v", x.lastRcvdData, elapsed)
		if elapsed > time.Minute*time.Duration(config.Xmpp.ActivityWatchdogMinutes) {
			message := fmt.Sprintf("WARNING: We haven't received anything for %v. We're quitting, hoping to be restarted !", elapsed)
			log.Printf(message)
			panic(message)
		}
	}
}

func (x *FtsXmpp) Start() {
	go x.runMain()  // Handles connection and fetches incoming messages
	go x.runRecv()  // Handles incoming messages
	go x.runSend()  // Sends messages
	go x.runCheck() // Check if the program behaves correctly
}

func (x *FtsXmpp) Stop() {
}
