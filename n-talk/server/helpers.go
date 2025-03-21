package main

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/charmbracelet/log"
)

// Modified from:
// https://gist.github.com/jerblack/4b98ba48ed3fb1d9f7544d2b1a1be287
func logOutput(log *log.Logger, errors chan<- error) func() {
	logFile := fmt.Sprintf("../outputs/server_log_%s.log", time.Now().Format(time.RFC3339))

	// open file read/write | create if not exist | clear file at open if exists
	f, _ := os.OpenFile(logFile, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o666)

	// save existing stdout | MultiWriter writes to saved stdout and file
	out := os.Stdout
	mw := io.MultiWriter(out, f)

	// Create a pipe. A pipe can be written to and read from. It is represented
	// in the OS as a pair of two connected files.

	r, w, err := os.Pipe()
	if err != nil {
		errors <- err
	}

	// Set the Standard Output and Standard Error outputs to point to the pipe's
	// `w`, which is the file that can be written to.

	os.Stdout = w
	os.Stderr = w

	// Set the logger's output to the MultiWriter, rather than Stdout. Keep in
	// mind this will remove color output from stdout. This is because
	// MultiWriter removes terminal color escape sequences. See:
	// https://github.com/sirupsen/logrus/issues/780#issuecomment-401542420

	log.SetOutput(mw)

	// Create a new channel `exit` with an empty struct type. This type
	// allocates no memory, because it is an empty struct. Also, this
	// empty struct channel pattern is idiomatic in Go for channels that
	// are used for signaling rather than value passing.
	//
	// See: https://dave.cheney.net/2014/03/25/the-empty-struct and also
	// check out: https://quii.gitbook.io/learn-go-with-tests/go-fundamentals/select#synchronising-processes

	exit := make(chan struct{})

	/// FUNCTION A ///

	go func() {
		// Read from the pipe's read file and copy it to the MultiWriter. The
		// MultiWriter will then write the copied data to wherever it is
		// configured to write to.
		//
		// Remember that `io.Copy` is a blocking function that only returns
		// when either an error is returned or it has reached EOF. So...

		_, err = io.Copy(mw, r)

		// ... using the `.Close()` method on either the io.Writer (mw) or the
		// io.Reader (r) makes `io.Copy(mw, r)` function return. After the
		// `io.Copy` function returns, `close(exit)` will execute, closing the
		// `exit` channel that was created earlier. Closing the `exit` channel
		// is a way to notify that logging operations are completed.

		close(exit)

		if err != nil {
			errors <- err
		}
	}()

	/// FUNCTION B ///

	// The return value is a function that should be executed at the end of the
	// program's lifetime (use `defer`).

	return func() {
		// Close the pipe's writer file from earlier. When this file is closed,
		// `io.Copy` from FUNCTION A will detect an EOF and subsequently return.

		err := w.Close()
		if err != nil {
			errors <- err
		}

		// After the pipe's writer file is closed, the bare `<-exit` is used to
		// block execution. `<-exit` will stop blocking if and only if the
		// `exit` channel receives a value, or, in this case, the `exit` channel
		// is closed (using `close(exit)`). Therefore, everything waits for
		// either of these two conditions.

		<-exit

		// Close the logging file that was being written to.

		err = f.Close()
		if err != nil {
			errors <- err
		}
	}
}
