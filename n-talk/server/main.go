package main

import (
	"log"
	"net"

	pb "codeberg.org/n30w/jasima/n-talk/chat"
	"google.golang.org/grpc"
)

func main() {
	fn := logOutput()
	defer fn()

	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	s := grpc.NewServer()

	chatServer := newChatServer()
	pb.RegisterChatServiceServer(s, chatServer)

	err = s.Serve(lis)
	if err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
