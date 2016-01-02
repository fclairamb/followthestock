package main

import (
	"testing"
)

// It's never too late to add unit tests
func TestParameters(t *testing.T) {
	db := NewFtsDB()
	defer db.Close()

	if err := db.SetParameter("test1", "value1"); err != nil {
		t.Fatal(err)
	}

	if value := db.GetParameter("test2"); value != nil {
		t.Fatalf(`value should be nil, it's "%s"`, value)
	}

	if value := db.GetParameter("test1"); value == nil || *value != "value1" {
		t.Fatalf(`value should be "test1", it's "%s"`, value)
	}

}

func TestStockDeletion(t *testing.T) {
	db := NewFtsDB()
	defer db.Close()

	{ // Data insertion
		s := &Stock{
			Market: "FR",
			Short:  "RNO",
		}

		if err := db.SaveStock(s); err != nil {
			t.Fatal("Problem", err)
		}

		c := &Contact{
			Email: "florent@clairambault.fr",
		}

		db.SaveContact(c)

		a := &Alert{
			Stock:   s.Id,
			Contact: c.Id,
		}

		db.SaveAlert(a)
	}

	{ // Data check and deletion
		s := db.GetStock("FR", "RNO")
		if s == nil || s.Id == 0 {
			t.Fatalf("Wrong stock: %#v", s)
		}

		db.DeleteStock(s)
	}

	{ // Data deletion
		s := db.GetStock("FR", "RNO")
		if s != nil {
			t.Fatalf("We should not have a stock anymore: %#v", s)
		}
	}
}
