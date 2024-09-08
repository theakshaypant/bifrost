package server

import (
	context "context"
	"fmt"
	"log"
	"net"

	"github.com/theakshaypant/bifrost/pkg/proto"
	"google.golang.org/grpc"
)

type serverObj struct {
	proto.UnimplementedBifrostServiceServer
	passcode string
	// TODO: Update to use a database to store users.
	users map[string]string
	// TODO: Update to use MQ for messages.
	messages map[string]([]*proto.Message)
}

var _ proto.BifrostServiceServer = serverObj{}

func StartBifrostServer(portNumber int, serverPasscode string) error {
	if portNumber < 1 || portNumber > 65535 {
		return fmt.Errorf("invalid port number to listen on! Value needs to be between 1 and 65535")
	}

	listen, err := net.Listen("tcp", fmt.Sprintf(":%d", portNumber))
	if err != nil {
		return fmt.Errorf("failed to listen on port %d: %s", portNumber, err)
	}

	grpcServer := grpc.NewServer()
	proto.RegisterBifrostServiceServer(grpcServer, serverObj{
		passcode: serverPasscode,
	})

	if err := grpcServer.Serve(listen); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}

	return nil
}

func (s serverObj) Join(ctx context.Context, req *proto.JoinRequest) (*proto.JoinResponse, error) {
	// Check if the server passcode is valid.
	if req.Passcode != s.passcode {
		return &proto.JoinResponse{
			Response: "Invalid server passcode!",
		}, nil
	}

	// Check if the username is unique.
	if _, ok := s.users[req.Username]; ok {
		return &proto.JoinResponse{
			Response: "Username already exists!",
		}, nil
	}

	// Add the user to the list of users.
	s.users[req.Username] = req.PublicKey

	return &proto.JoinResponse{
		Response: "Ok!",
	}, nil
}
func (s serverObj) SendPreprepare(ctx context.Context, req *proto.SendPreprepareRequest) (*proto.SendPreprepareResponse, error) {
	// Check if username is valid.
	if _, ok := s.users[req.Username]; !ok {
		return &proto.SendPreprepareResponse{
			Response: "Invalid username!",
		}, nil
	}

	// Fetch the users public key if the username is valid.
	publicKey := s.users[req.Username]

	return &proto.SendPreprepareResponse{
		PublicKey: publicKey,
		Response:  "Ok!",
	}, nil
}
func (s serverObj) Send(ctx context.Context, req *proto.SendRequest) (*proto.SendResponse, error) {
	// Check if username is valid.
	if _, ok := s.users[req.Msg.Sender]; !ok {
		return &proto.SendResponse{
			Response: "Invalid username!",
		}, nil
	}
	// Check if receiver is valid.
	if _, ok := s.users[req.Receiver]; !ok {
		return &proto.SendResponse{
			Response: "Invalid receiver username!",
		}, nil
	}

	// Add the message to the list of messages.
	s.messages[req.Msg.Sender] = append(s.messages[req.Msg.Sender], req.Msg)

	return &proto.SendResponse{Response: "Ok!"}, nil
}

func (s serverObj) Fetch(req *proto.FetchRequest, msgStream grpc.ServerStreamingServer[proto.FetchResponse]) error {
	// Check if username is valid.
	if _, ok := s.users[req.Username]; !ok {
		msgStream.Send(&proto.FetchResponse{
			Response: "Invalid username!",
		})
	}

	// Fetch the messages for the user.
	messages := s.messages[req.Username]
	for _, msg := range messages {
		if err := msgStream.Send(&proto.FetchResponse{
			Message: msg,
		}); err != nil {
			log.Println("error generating response")
			return err
		}

	}

	return nil
}
