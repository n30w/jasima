package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"syscall"

	pb "codeberg.org/n30w/jasima/n-talk/chat"
	"codeberg.org/n30w/jasima/n-talk/memory"
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

	// conn, err := client.Connect(ctx)
	// if err != nil {
	// 	log.Fatalf("did not connect: %v", err)
	// }

	fmt.Println("Making new client...")
	connection, err := grpc.NewClient(client.config.server, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("%v", err)
	}

	defer connection.Close()

	fmt.Println("new client created")
	c := pb.NewChatServiceClient(connection)
	conn, err := c.Chat(ctx)
	if err != nil {
		log.Fatalf("could not create stream: %v", err)
	}
	fmt.Println("stream	client created")

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	// Handshake
	err = conn.Send(&pb.Message{
		Sender: *name,
		// Receiver: *recipient,
		Content: "HANDSHAKE",
	})
	if err != nil {
		log.Fatalf("failed handshake %v", err)
	}

	// Send messages
	go func() {
		scanner := bufio.NewScanner(os.Stdin)
		for {
			fmt.Print("> ") // Prompt for user input
			if scanner.Scan() {
				text := scanner.Text()
				if text == "exit" {
					fmt.Println("Closing connection...")
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

	// Receive messages
	// Anything that is received is sent to the LLM.
	go func() {
		for {
			msg, err := conn.Recv()
			if err == io.EOF {
				break
				// Send the data to the LLM.

				// When data is received back from the query,
				// fill the channel.
			}
			if err != nil {
				log.Fatalf("failed to receive message: %v", err)
			}
			fmt.Printf("%s: %s\n> ", msg.Sender, msg.Content)
		}
	}()

	<-stop

	fmt.Println("shutting down")
	conn.CloseSend()
}
