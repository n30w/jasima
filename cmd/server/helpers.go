package main

import (
	"io"
	"os"

	"github.com/charmbracelet/log"
)

// logOutput uses a logger to write outputs to both stdout and a file at
// `logFilePath`. logOutput writes to the error channel `errors` when an error
// occurs.
//
// This function is a modified version of:
// https://gist.github.com/jerblack/4b98ba48ed3fb1d9f7544d2b1a1be287
// This implementation uses an error channel and contains more documentation
// for learning purposes.
//
// For a NON-annotated version of this function, see:
// https://gist.github.com/n30w/bb7e1ab90838b398bba863ca486c1344#file-tee_with_channel-go
func logOutput(
	log *log.Logger,
	logFilePath string,
	errors chan<- error,
) func() {
	// Create a new file at the specified `logFile` location. If the file
	// does not exist, it creates a new one. Otherwise, it will clear the file
	// of its contents when it opens.

	f, _ := os.OpenFile(logFilePath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o666)

	// Create a new MultiWriter, `mw`. MultiWriter, as the name suggests, writes
	// the output of multiple writers to different locations.

	out := os.Stdout
	mw := io.MultiWriter(out, f)

	// Create a pipe. A pipe can be written to and read from. It is represented
	// in the OS as a pair of two connected files, one that can be read from and
	// one that can be written to.

	r, w, err := os.Pipe()
	if err != nil {
		errors <- err
	}

	// Set stdout and stderr to point to the pipe's write file, `w`.

	os.Stdout = w
	os.Stderr = w

	// Set the logger's output to the MultiWriter, rather than stdout.
	//
	// Keep in mind this will remove color output from stdout. This is because
	// MultiWriter removes terminal color escape sequences. See:
	// https://github.com/sirupsen/logrus/issues/780#issuecomment-401542420

	log.SetOutput(mw)

	// Create a new channel, `exit`. The type of `exit` is an empty struct.
	// This type allocates no memory, because it is an empty struct. The empty
	// struct channel pattern is idiomatic in Go for channels that are used for
	// signaling of events rather than sharing of data.
	//
	// See:
	// - https://dave.cheney.net/2014/03/25/the-empty-struct
	// - https://quii.gitbook.io/learn-go-with-tests/go-fundamentals/select#synchronising-processes

	exit := make(chan struct{})

	// This next section contains two functions, FUNCTIOn A and FUNCTION B.
	// FUNCTION A is dispatched into its own go routine. FUNCTION B is the return
	// value of the function enclosing the return keyword.

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
		// io.Reader (r) makes the `io.Copy(mw, r)` function return, because
		// inside io.Copy's implementation, there is code that will: 1) exit the
		// function when `mw` or `r` returns an error; OR 2) exit the function when
		// `r` has reached io.EOF.

		// Now, after the `io.Copy` function returns, `close(exit)` will execute...

		close(exit)

		// ... which closes the `exit` channel that was created earlier. Remember
		// that closing the `exit` channel is a way to notify that logging
		// operations are now complete.

		if err != nil {
			errors <- err
		}
	}()

	/// FUNCTION B ///

	// The return value is a function that should be executed at the end of the
	// enclosing function's caller's lifetime (use `defer`).

	return func() {
		// Close the pipe's writer file that was defined from earlier. When
		// this file is closed, `io.Copy` from FUNCTION A will detect an EOF
		// and subsequently return, stopping FUNCTION A's go routine.

		err := w.Close()
		if err != nil {
			errors <- err
		}

		// After the pipe's writer file is closed, the bare `<-exit` is used to
		// block execution. To "block execution" means the code that immediately
		// follows AFTER `<-exit` will not execute. `<-exit` will stop blocking if
		// and only if the `exit` channel receives a value, or, in this case, the
		// `exit` channel is closed (using `close(exit)`). In a slightly more formal
		// syntax:
		//
		// close(exit) + send_value_to_exit(value) => ~block.

		<-exit

		// Make sure to close the logging file that `mw` was writing to.

		err = f.Close()
		if err != nil {
			errors <- err
		}
	}
}
