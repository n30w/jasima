package main

import (
	"fmt"
	"log"
	"net"

	pb "codeberg.org/n30w/jasima/n-talk/chat"
	"google.golang.org/grpc"
)

type chatServer struct {
	pb.UnimplementedChatServiceServer
}

func (s *chatServer) Chat(stream pb.ChatService_ChatServer) error {
	for {
		msg, err := stream.Recv()
		if err != nil {
			return err
		}
		fmt.Printf("Received: %v\n", msg)

		// Simulate processing and sending a response
		response := &pb.Message{
			Sender:  "Server",
			Content: "Echo: " + msg.Content,
		}
		if err := stream.Send(response); err != nil {
			return err
		}
		fmt.Printf("Sent: %v\n", response)
	}
}

func main() {
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	s := grpc.NewServer()
	pb.RegisterChatServiceServer(s, &chatServer{})
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
