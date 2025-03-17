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
	"github.com/BurntSushi/toml"
	"github.com/charmbracelet/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type ModelConfig struct {
	Provider     int
	Instructions string
	Temperature  float64
	Initialize   string
}

type NetworkConfig struct {
	Router   string
	Database string
}

type ConfigFile struct {
	Name      string
	Recipient string
	Model     ModelConfig
	Network   NetworkConfig
}

func main() {

	var err error

	flagName := flag.String("name", "", "name of the agent")
	flagRecipient := flag.String("recipient", "", "name of the recipient agent")
	flagServer := flag.String("server", "", "communication server")
	flagProvider := flag.Int("model", -1, "LLM model to use")
	configPath := flag.String("configFile", "./configs/a1.toml", "configuration file path")

	flag.Parse()

	var conf ConfigFile
	_, err = toml.DecodeFile(*configPath, &conf)
	if err != nil {
		log.Fatal(err)
	}

	name := conf.Name
	recipient := conf.Recipient
	router := conf.Network.Router
	provider := conf.Model.Provider

	if *flagName != "" {
		name = *flagName
	}

	if *flagRecipient != "" {
		recipient = *flagRecipient
	}

	if *flagServer != "" {
		router = *flagServer
	}

	if *flagProvider != -1 {
		provider = *flagProvider
		conf.Model.Provider = *flagProvider
	}

	ctx := context.Background()
	memory := memory.NewMemoryStore()

	llm, err := selectModel(ctx, conf.Model)
	if err != nil {
		log.Fatal(err)
	}

	logger := log.NewWithOptions(os.Stderr, log.Options{
		ReportCaller:    true,
		ReportTimestamp: true,
	})

	cfg := &config{
		name:   name,
		server: router,
		model:  llms.LLMProvider(provider),
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

	log.Info("Created new agent!", "name", name, "server", router, "model", llm)

	c := pb.NewChatServiceClient(connection)

	// The implementation of `Chat` is in the server code. This is where the
	// client establishes a first connection to the server.
	conn, err := c.Chat(ctx)
	if err != nil {
		log.Fatalf("could not create stream: %v", err)
	}

	// Initialize a connection.
	err = conn.Send(&pb.Message{
		Sender:   name,
		Receiver: recipient,
		Content:  client.model.String(),
	})
	if err != nil {
		log.Fatalf("Failed to send message: %v", err)
	}

	log.Info("Established connection to server")

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	responseChan := make(chan string)

	// Send the initialize SVG message.
	if conf.Model.Initialize != "" {
		file, err := os.Open(conf.Model.Initialize)
		if err != nil {
			log.Fatalf("Failed to open file: %v", err)
		}
		defer file.Close()

		data, err := io.ReadAll(file)
		if err != nil {
			log.Fatalf("Failed to read file: %v", err)
		}

		svgText := string(data)

		client.memory.Save(ctx, client.NewMessageTo(recipient, svgText))
		err = conn.Send(&pb.Message{
			Sender:   name,
			Receiver: recipient,
			Content:  svgText,
		})
		if err != nil {
			log.Fatalf("Failed to send message: %v", err)
		}
	}

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
					client.memory.Save(ctx, client.NewMessageTo(recipient, text))
				}

				err := conn.Send(&pb.Message{
					Sender:   name,
					Receiver: recipient,
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
				Sender:   name,
				Receiver: recipient,
				Content:  response,
			})
			if err != nil {
				log.Fatalf("Failed to send response: %v", err)
			}

			// log.Printf("YOU%s: %s\n", client.model, response)
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

			// log.Printf("%s: %s\n> ", msg.Sender, msg.Content)

			// Send the data to the LLM.
			go func(msg *pb.Message) {

				// When data is received back from the query,
				// fill the channel.

				client.memory.Save(ctx, client.NewMessageFrom(msg.Receiver, msg.Content))

				if client.model != llms.ProviderOllama {
					time.Sleep(time.Second * 18)
				}

				time.Sleep(time.Second * 2)

				// log.Info("Dispatched message to LLM")

				res, err := client.Request(ctx, msg.Content)
				if err != nil {
					log.Fatal(err)
				}

				// Save the response to memory.

				client.memory.Save(ctx, client.NewMessageTo(msg.Sender, msg.Content))

				if client.model != llms.ProviderOllama {
					time.Sleep(time.Second * 18)
				}

				time.Sleep(time.Second * 2)

				responseChan <- res

			}(msg)
		}
	}()

	<-stop

	log.Info("shutting down")
	conn.CloseSend()
}
