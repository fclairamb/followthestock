package main

import (
	"testing"
)

// It's never too late to add unit tests
func TestParameters(t *testing.T) {
	config = NewConfig()

	db := NewFtsDB()

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
