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
	"codeberg.org/n30w/jasima/n-talk/memory"
	"github.com/charmbracelet/log"
	"github.com/joho/godotenv"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const ModelName = "gemini-2.0-flash"

// const Prompt = "I'm using the Google Gemini API. I'm trying to make sure that every time I send a query, the model remembers what we were talking about before. How do I do this? I'm using Go, not Python."

const Prompt = "Give me a list of cool verbs."

func main() {
	name := flag.String("name", "toki", "name of the agent")
	recipient := flag.String("recipient", "pona", "name of the recipient agent")
	server := flag.String("server", "localhost:50051", "communication server")
	model := flag.Int("model", 1, "LLM model to use")

	flag.Parse()

	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	apiKey := os.Getenv("LLM_API_KEY")
	memory := memory.NewMemoryStore()
	ctx := context.Background()

	llm, err := selectModel(ctx, apiKey, *model)
	if err != nil {
		log.Fatal(err)
	}

	cfg := &config{
		name:   *name,
		server: *server,
	}

	client, err := NewClient(ctx, llm, memory, cfg)
	if err != nil {
		log.Fatal(err)
	}

	// res, err := client.Request(ctx, Prompt)
	// if err != nil {
	// 	log.Fatal(err)
	// }

	// fmt.Println(res)

	log.Info("Making new client...", "name", *name, "server", *server, "model", *model)

	connection, err := grpc.NewClient(client.config.server, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("%v", err)
	}

	defer connection.Close()

	log.Info("New client created")

	c := pb.NewChatServiceClient(connection)

	// The implementation of `Chat` is in the server code. This is where the
	// client establishes a first connection to the server.
	conn, err := c.Chat(ctx)
	if err != nil {
		log.Fatalf("could not create stream: %v", err)
	}

	log.Info("stream client created")

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

				time.Sleep(time.Second * 30)

				log.Info("Dispatched message to LLM")

				res, err := client.Request(ctx, receivedMsg)
				if err != nil {
					log.Fatal(err)
				}

				time.Sleep(time.Second * 30)

				responseChan <- res

			}(msg.Content)

		}
	}()

	<-stop

	log.Info("shutting down")
	conn.CloseSend()
}
