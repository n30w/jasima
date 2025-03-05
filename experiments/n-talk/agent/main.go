package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	pb "codeberg.org/n30w/jasima/n-talk/chat"
	"codeberg.org/n30w/jasima/n-talk/llms"
	"codeberg.org/n30w/jasima/n-talk/memory"
	"github.com/charmbracelet/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {

	var err error

	name := flag.String("name", "toki", "name of the agent")
	recipient := flag.String("recipient", "pona", "name of the recipient agent")
	server := flag.String("server", "localhost:50051", "communication server")
	model := flag.Int("model", 0, "LLM model to use")

	flag.Parse()

	ctx := context.Background()
	memory := memory.NewMemoryStore()

	llm, err := selectModel(ctx, *model)
	if err != nil {
		log.Fatal(err)
	}

	logger := log.NewWithOptions(os.Stderr, log.Options{
		ReportCaller:    true,
		ReportTimestamp: true,
	})

	cfg := &config{
		name:   *name,
		server: *server,
		model:  llms.LLMProvider(*model),
	}

	client, err := NewClient(ctx, llm, memory, cfg, logger)
	if err != nil {
		log.Fatal(err)
	}

	connection, err := grpc.NewClient(client.config.server, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("%v", err)
	}

	defer connection.Close()

	log.Info("Created new agent!", "name", *name, "server", *server, "model", llm)

	c := pb.NewChatServiceClient(connection)

	// The implementation of `Chat` is in the server code. This is where the
	// client establishes a first connection to the server.
	conn, err := c.Chat(ctx)
	if err != nil {
		log.Fatalf("could not create stream: %v", err)
	}

	log.Info("Established connection to server")

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	responseChan := make(chan string)

	// Send messages
	go func() {
		scanner := bufio.NewScanner(os.Stdin)
		for {

			fmt.Print("> ") // Prompt for user input

			if scanner.Scan() {

				text := scanner.Text()

				if text == "exit" {
					log.Info("Closing connection...")
					conn.CloseSend()
					return
				}

				// Save as model, as we are "talking" as the model.
				// If the user presses "enter" with nothing written, don't
				// save anything!

				if text != "" {
					client.memory.Save(1, text)
				}

				err := conn.Send(&pb.Message{
					Sender:   *name,
					Receiver: *recipient,
					Content:  text,
				})
				if err != nil {
					log.Fatalf("Failed to send message: %v", err)
				}

			} else {
				// Handle scanner errors
				if err := scanner.Err(); err != nil {
					log.Fatalf("Error reading input: %v", err)
				}
			}
		}
	}()

	go func() {
		for response := range responseChan {

			err := conn.Send(&pb.Message{
				Sender:   *name,
				Receiver: *recipient,
				Content:  response,
			})
			if err != nil {
				log.Fatalf("Failed to send response: %v", err)
			}

			log.Printf("YOU: %s\n", response)
		}
	}()

	// Receive messages
	// Anything that is received is sent to the LLM.
	go func() {
		for {

			msg, err := conn.Recv()

			if err == io.EOF {
				// This exits the program when the connection is terminated
				// by the server.
				break
			}

			if err != nil {
				log.Fatalf("failed to receive message: %v", err)
			}

			log.Printf("%s: %s\n> ", msg.Sender, msg.Content)

			// Send the data to the LLM.
			go func(receivedMsg string) {

				// When data is received back from the query,
				// fill the channel.

				client.memory.Save(0, receivedMsg)

				if client.model != llms.ProviderOllama {
					time.Sleep(time.Second * 18)
				}

				time.Sleep(time.Second * 2)

				// log.Info("Dispatched message to LLM")

				res, err := client.Request(ctx, receivedMsg)
				if err != nil {
					log.Fatal(err)
				}

				// Save the response to memory.

				client.memory.Save(1, res)

				if client.model != llms.ProviderOllama {
					time.Sleep(time.Second * 18)
				}

				time.Sleep(time.Second * 2)

				responseChan <- res

			}(msg.Content)
		}
	}()

	<-stop

	log.Info("shutting down")
	conn.CloseSend()
}
