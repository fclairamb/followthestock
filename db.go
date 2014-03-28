package main

import (
	"database/sql"
	"errors"
	"github.com/coopernurse/gorp"
	_ "github.com/mattn/go-sqlite3"
	"log"
	"strings"
)

type Stock struct {
	// db tag lets you specify the column name if it differs from the struct field
	Id     int64  `db:"stock_id"`
	Market string `db:"market"` // "fr", "us"
	Short  string `db:"short"`
	Name   string `db:"name"`
}

type Contact struct {
	Id         int64  `db:"contact_id"`
	Email      string `db:"email"`
	PauseUntil int64  `db:"pause_until"`
}

type Value struct {
	Id    int64   `db:"value_id"`
	Stock int64   `db:"stock_id"`
	Date  int64   `db:"date"`
	Value float32 `db:"value"`
}

type Alert struct {
	Id            int64   `db:"alert_id"`
	Contact       int64   `db:"contact_id"`
	Stock         int64   `db:"stock_id"`
	LastTriggered int64   `db:"last_triggered"`
	LastValue     float32 `db:"last_value"`
	Percent       float32 `db:"percent"`
}

type FtsDB struct {
	connection *sql.DB
	mapping    *gorp.DbMap
}

func NewFtsDB(par *Parameters) *FtsDB {
	// We connect to the database
	conn, err := sql.Open("sqlite3", par.dbfile)
	if err != nil {
		log.Fatal(err)
	}

	// We create the DbMap instance
	dbmap := &gorp.DbMap{Db: conn, Dialect: gorp.SqliteDialect{}}

	// We register the tables
	dbmap.AddTableWithName(Stock{}, "stock").SetKeys(true, "Id")
	dbmap.AddTableWithName(Contact{}, "contact").SetKeys(true, "Id")
	dbmap.AddTableWithName(Value{}, "value").SetKeys(true, "Id")
	dbmap.AddTableWithName(Alert{}, "alert").SetKeys(true, "Id")

	// We create the tables
	err = dbmap.CreateTablesIfNotExists()
	if err != nil {
		log.Fatal(err)
	}

	return &FtsDB{connection: conn, mapping: dbmap}
}

func (db FtsDB) Close() {
	db.connection.Close()
}

func (db *FtsDB) GetContactFromEmail(email string) *Contact {
	// We remove the part after the "/"
	email = strings.SplitN(email, "/", 2)[0]

	c := &Contact{}
	err := db.mapping.SelectOne(c, "select * from contact where email=?", email)
	if err != nil {
		log.Println("Creating contact ", email)
		c.Email = email
		err := db.mapping.Insert(c)
		if err != nil {
			log.Println("Could not insert:", err)
		}
	}

	return c
}

func (db *FtsDB) GetContactFromId(id int64) *Contact {
	c := &Contact{}
	err := db.mapping.SelectOne(c, "select * from contact where contact_id=?", id)
	if err != nil {
		c = nil
	}
	return c
}

func (db *FtsDB) DeleteContact(c *Contact) (err error) {
	_, err = db.mapping.Delete(c)
	return
}

func (db *FtsDB) GetStock(market, short string) *Stock {
	log.Printf("GetStock( \"%s\", \"%s\" );", market, short)
	s := &Stock{}
	err := db.mapping.SelectOne(s, "select * from stock where market=? and short=?", market, short)

	if err != nil {
		return nil
	} else {
		return s
	}
}

func (db *FtsDB) GetStockFromId(id int64) *Stock {
	s := &Stock{}
	err := db.mapping.SelectOne(s, "select * from stock where stock_id=?", id)
	if err != nil {
		return nil
	} else {
		return s
	}
}

func (db *FtsDB) GetAlertsForStock(s *Stock) *[]Alert {
	var alerts []Alert
	db.mapping.Select(&alerts, "select * from alert where stock_id=?", s.Id)
	return &alerts
}

func (db *FtsDB) GetAllStocks() *[]Stock {
	var stocks []Stock
	db.mapping.Select(&stocks, "select * from stock")
	return &stocks
}

func (db *FtsDB) SaveStockValue(stock *Stock, value float32) {

}

func (db *FtsDB) SubscribeAlert(s *Stock, c *Contact, per float32) (alert *Alert, err error) {
	_, err = db.UnsubscribeAlert(s, c)

	if err != nil {
		return nil, err
	}

	alert = &Alert{Stock: s.Id, Contact: c.Id, Percent: per}

	err = db.SaveAlert(alert)

	return
}

func (db *FtsDB) UnsubscribeAlert(s *Stock, c *Contact) (ok bool, err error) {
	_, err = db.mapping.Exec("delete from alert where stock_id=? and contact_id=?", s.Id, c.Id)
	return
}

func (db *FtsDB) SaveContact(c *Contact) (err error) {
	if c.Id != 0 {
		_, err = db.mapping.Update(c)
	} else {
		err = errors.New("Contact doesn't exist !")
	}
	return
}

func (db *FtsDB) SaveAlert(a *Alert) (err error) {
	if a.Id != 0 {
		_, err = db.mapping.Update(a)
	} else {
		err = db.mapping.Insert(a)
	}
	return
}

func (db *FtsDB) GetAlertsForContact(c *Contact) *[]Alert {
	var alerts []Alert
	db.mapping.Select(&alerts, "select * from alert where contact_id=?", c.Id)
	return &alerts
}

func (db *FtsDB) DeleteAlert(a *Alert) (err error) {
	_, err = db.mapping.Delete(a)
	return
}

func (df *FtsDB) SaveStock(s *Stock) error {
	if s.Id != 0 {
		_, e := db.mapping.Update(s)
		return e
	} else {
		return db.mapping.Insert(s)
	}
}

func (s *Stock) String() string {
	return "\"" + s.Name + "\" (" + s.Market + ":" + s.Short + ")"
}
