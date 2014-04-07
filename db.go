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
	Id       int64   `db:"stock_id"`
	Market   string  `db:"market"` // "FR","US","W"
	Short    string  `db:"short"`
	Name     string  `db:"name"`
	Value    float32 `db:"value"` // Last value
	Currency string  `db:"currency"`
}

type CurrencyConversion struct {
	From       string  `db:"from"`
	To         string  `db:"to"`
	Rate       float32 `db:"rate"`
	LastUpdate int64   `db:"last_update"`
}

type ContactStockValue struct {
	Id      int64   `db:"stock_value_id"`
	Contact int64   `db:"contact_id"`
	Stock   int64   `db:"stock_id"`
	Nb      int32   `db:"nb"`
	Value   float32 `db:"value"`
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

const (
	TABLE_STOCK               = "stock"
	TABLE_CONTACT             = "contact"
	TABLE_VALUE               = "value"
	TABLE_ALERT               = "alert"
	TABLE_CONTACT_STOCK_VALUE = "contactstockvalue"
	TABLE_CURRENCY_CONVERSION = "currency_conversion"
)

func NewFtsDB() *FtsDB {
	// We connect to the database
	conn, err := sql.Open("sqlite3", par.dbfile)
	if err != nil {
		log.Fatal(err)
	}

	// We create the DbMap instance
	dbmap := &gorp.DbMap{Db: conn, Dialect: gorp.SqliteDialect{}}

	// We register the tables
	dbmap.AddTableWithName(Stock{}, TABLE_STOCK).SetKeys(true, "Id")
	dbmap.AddTableWithName(Contact{}, TABLE_CONTACT).SetKeys(true, "Id")
	dbmap.AddTableWithName(Value{}, TABLE_VALUE).SetKeys(true, "Id")
	dbmap.AddTableWithName(Alert{}, TABLE_ALERT).SetKeys(true, "Id")
	dbmap.AddTableWithName(CurrencyConversion{}, TABLE_CURRENCY_CONVERSION).SetUniqueTogether("from", "to")
	dbmap.AddTableWithName(ContactStockValue{}, TABLE_CONTACT_STOCK_VALUE).SetKeys(true, "Id")

	// We create the tables
	err = dbmap.CreateTablesIfNotExists()
	if err != nil {
		log.Fatal(err)
	}

	if false { // WAL is faster & safer
		_, err = conn.Exec("pragma journal_mode = wal")
		if err != nil {
			log.Fatal(err)
		}
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
	err := db.mapping.SelectOne(c, "select * from "+TABLE_CONTACT+" where email=?", email)
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
	err := db.mapping.SelectOne(c, "select * from "+TABLE_CONTACT+" where contact_id=?", id)
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
	//log.Printf("GetStock( \"%s\", \"%s\" );", market, short)
	s := &Stock{}
	err := db.mapping.SelectOne(s, "select * from "+TABLE_STOCK+" where market=? and short=?", market, short)

	if err != nil {
		return nil
	} else {
		return s
	}
}

func (db *FtsDB) GetStockFromId(id int64) *Stock {
	s := &Stock{}
	err := db.mapping.SelectOne(s, "select * from "+TABLE_STOCK+" where stock_id=?", id)
	if err != nil {
		return nil
	} else {
		return s
	}
}

func (db *FtsDB) GetAlertsForStock(s *Stock) *[]Alert {
	var alerts []Alert
	db.mapping.Select(&alerts, "select * from "+TABLE_ALERT+" where stock_id=?", s.Id)
	return &alerts
}

func (db *FtsDB) GetAllStocks() *[]Stock {
	var stocks []Stock
	db.mapping.Select(&stocks, "select * from "+TABLE_STOCK)
	return &stocks
}

func (db *FtsDB) SaveStockValue(stock *Stock, value float32) error {
	if stock.Value != value {
		stock.Value = value
		return db.SaveStock(stock)
	}
	return nil
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
	_, err = db.mapping.Exec("delete from "+TABLE_ALERT+" where stock_id=? and contact_id=?", s.Id, c.Id)
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
	db.mapping.Select(&alerts, "select * from "+TABLE_ALERT+" where contact_id=?", c.Id)
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

func (db *FtsDB) GetContactStockValue(contactId, stockId int64) *ContactStockValue {
	csv := &ContactStockValue{Contact: contactId, Stock: stockId}
	if err := db.mapping.SelectOne(csv, "select * from "+TABLE_CONTACT_STOCK_VALUE+" where contact_id=? and stock_id=?", csv.Contact, csv.Stock); err != nil {
		return nil
	}
	return csv
}

func (db *FtsDB) GetContactStockValuesFromContact(c *Contact) *[]ContactStockValue {
	var values []ContactStockValue
	db.mapping.Select(&values, "select * from "+TABLE_CONTACT_STOCK_VALUE+" where contact_id=?", c.Id)
	return &values
}

func (db *FtsDB) SaveContactStockValue(csv *ContactStockValue) error {
	if csv.Id != 0 {
		_, err := db.mapping.Update(csv)
		return err
	} else {
		return db.mapping.Insert(csv)
	}

}

func (db *FtsDB) DeleteContactStockValue(s *ContactStockValue) (err error) {
	_, err = db.mapping.Delete(s)
	return
}

func (db *FtsDB) GetCurrencyConversion(from, to string) *CurrencyConversion {
	c := &CurrencyConversion{}
	err := db.mapping.SelectOne(c, "select * from "+TABLE_CURRENCY_CONVERSION+" where from=? and to=?", from, to)
	if err == nil {
		return c
	} else {
		return nil
	}
}

func (db *FtsDB) SaveCurrencyConversion(c *CurrencyConversion) error {
	return db.mapping.Insert(c)
}

func (db *FtsDB) DeleteCurrencyConversion(c *CurrencyConversion) (err error) {
	_, err = db.mapping.Delete(c)
	return
}

func (s *Stock) String() string {
	return "\"" + s.Name + "\" (" + s.Market + ":" + s.Short + ")"
}
