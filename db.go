package main

import (
	"database/sql"
	"github.com/coopernurse/gorp"
	_ "github.com/mattn/go-sqlite3"
	"log"
)

type Stock struct {
	// db tag lets you specify the column name if it differs from the struct field
	Id    int64 `db:"stock_id"`
	Short string
	Name  string
}

type Contact struct {
	Id             int64  `db:"contact_id"`
	Email          string `db:"email"`
	NbMessagesSent int32
}

type Value struct {
	Id    int64 `db:"value_id"`
	Stock int64 `db:"stock_id"`
	Value float32
}

type Alert struct {
	Id      int64 `db:"alert_id"`
	Contact int64 `db:"contact_id"`
	Stock   int64 `db:"stock_id"`
	Diff    float32
}

type FtsDB struct {
	connection *sql.DB
	mapping    *gorp.DbMap
}

func FtsDBOpen() *FtsDB {
	// We connect to the database
	conn, err := sql.Open("sqlite3", "followthestock.db")
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

func (db FtsDB) GetContact(email string) *Contact {
	c := &Contact{}
	err := db.mapping.SelectOne(c, "select * from contact where email=?", email)
	if err != nil {
		log.Println("Creating contact ", email)
		c.Email = email
		err := db.mapping.Insert(c)
		log.Println("Could not insert:", err)
	}

	return c
}

type DbSubscribeStock struct {
	contact   string
	stock     string
	variation float32
}
