package main

import (
	"flag"
	"io"
	"log"
	"os"
	"time"
)

// const LOGS_FILE_OLD = LOGS_FILE + ".old"
// const LOGS_FILE_MAX_SIZE = 1024 * 1024 // 1MB

var file *os.File

var logFile string

func log_set_file() {
	var err error
	if file, err = os.OpenFile(logFile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0660); err != nil {
		log.Fatal("Could not open log file", logFile, " ! ", err)
	} else {
		log.SetOutput(io.MultiWriter(file, os.Stdout))
	}
}

func init() {
	flag.StringVar(&logFile, "logfile", "followthestock.log", "Log file")

	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds | log.Lshortfile)
	log_set_file()

	ticker := time.NewTicker(time.Hour)
	go func() {
		for {
			<-ticker.C // This is what makes us wait
			log_set_file()

			// We actually don't want to do rotate file ourselves but we
			// do want to let an other program (like logrotate) handle it correctly.
			// if file != nil {
			// 	if fi, err := file.Stat(); err == nil {
			// 		if fi.Size() > LOGS_FILE_MAX_SIZE {
			// 			log.Println("We have a size of", fi.Size(), ": Rotating logfile...")
			// 			file.Close()
			// 			os.Remove(LOGS_FILE_OLD)
			// 			os.Rename(LOGS_FILE, LOGS_FILE_OLD)
			// 			log_set_file()
			// 		}
			// 	}
			// }
		}
	}()
}
