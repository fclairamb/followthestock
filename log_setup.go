package main

import (
	"io"
	"log"
	"os"
	"time"
)

const LOGS_FILE = "followthestock.log"
const LOGS_FILE_OLD = LOGS_FILE + ".old"
const LOGS_FILE_MAX_SIZE = 1024 * 1024 // 1MB

var file *os.File

func log_set_file() {
	var err error
	if file, err = os.OpenFile(LOGS_FILE, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0660); err != nil {
		log.Fatal("Could not open log file ! ", err)
	} else {
		log.SetOutput(io.MultiWriter(file, os.Stdout))
	}
}

func init() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds | log.Lshortfile)
	log_set_file()

	ticker := time.NewTicker(time.Minute)
	go func() {
		for {
			<-ticker.C // This is what makes us wait
			if file != nil {
				if fi, err := file.Stat(); err == nil {
					if fi.Size() > LOGS_FILE_MAX_SIZE {
						log.Println("We have a size of", fi.Size(), ": Rotating logfile...")
						file.Close()
						os.Remove(LOGS_FILE_OLD)
						os.Rename(LOGS_FILE, LOGS_FILE_OLD)
						log_set_file()
					}
				}
			}
		}
	}()
}
