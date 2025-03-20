package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"time"
)

// https://gist.github.com/jerblack/4b98ba48ed3fb1d9f7544d2b1a1be287
func logOutput() func() {
	logFile := fmt.Sprintf("../outputs/server_log_%s.log", time.Now().Format(time.RFC3339))
	// open file read/write | create if not exist | clear file at open if exists
	f, _ := os.OpenFile(logFile, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o666)

	// save existing stdout | MultiWriter writes to saved stdout and file
	out := os.Stdout
	mw := io.MultiWriter(out, f)

	// get pipe reader and writer | writes to pipe writer come out pipe reader
	r, w, _ := os.Pipe()

	// replace stdout,stderr with pipe writer | all writes to stdout, stderr will go through pipe instead (fmt.print, log)
	os.Stdout = w
	os.Stderr = w

	// writes with log.Print should also write to mw
	log.SetOutput(mw)

	// create channel to control exit | will block until all copies are finished
	exit := make(chan bool)

	go func() {
		// copy all reads from pipe to multiwriter, which writes to stdout and file
		_, _ = io.Copy(mw, r)
		// when r or w is closed copy will finish and true will be sent to channel
		exit <- true
	}()

	// function to be deferred in main until program exits
	return func() {
		// close writer then block on exit channel | this will let mw finish writing before the program exits
		_ = w.Close()
		<-exit
		// close file after all writes have finished
		_ = f.Close()
	}
}
