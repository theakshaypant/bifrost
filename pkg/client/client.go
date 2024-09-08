package client

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"io"
	"log"
	"time"

	"github.com/theakshaypant/bifrost/pkg/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type clientObj struct {
	proto.BifrostServiceClient
	username string
	// Private key
	privateKey *rsa.PrivateKey
}

func NewBifrostServiceClient(serverAddress string) clientObj {
	// TODO: Change this to not allow insecure connections.
	conn, err := grpc.NewClient(
		serverAddress,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Fatalf("failed to connect to gRPC server at %s: %v", serverAddress, err)
	}

	biforstClient := proto.NewBifrostServiceClient(conn)

	return clientObj{
		BifrostServiceClient: biforstClient,
	}
}

func (c clientObj) Join(serverPassword, username string) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	// Generate private key for the user.
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		log.Fatalf("failed to generate private key for user: %v", err)
	}
	publicKey := &privateKey.PublicKey
	publicKeyBytes, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		panic(err)
	}

	r, err := c.BifrostServiceClient.Join(ctx, &proto.JoinRequest{
		Username:  username,
		Passcode:  serverPassword,
		PublicKey: string(publicKeyBytes),
	})
	if err != nil {
		log.Fatalf("error calling function SayHello: %v", err)
	}

	if r.Response != "OK!" {
		log.Fatalf("failed to join server: %s", r.Response)
	}

	c.username = username
}

func (c clientObj) Send(message, receiver string) {
	// Fetch the public key for the receiver.
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	r, err := c.BifrostServiceClient.SendPreprepare(ctx, &proto.SendPreprepareRequest{
		Username: receiver,
	})
	if err != nil {
		log.Fatalf("error calling function SendPreprepare: %v", err)
	}
	if r.Response != "OK!" {
		log.Fatalf("failed to join server: %s", r.Response)
	}

	// Encrypt the message with the receiver's public key.
	receiverPublicKey, err := x509.ParsePKIXPublicKey([]byte(r.PublicKey))
	if err != nil {
		log.Fatalf("failed to parse public key: %v", err)
	}

	cipherText, err := rsa.EncryptPKCS1v15(rand.Reader, receiverPublicKey.(*rsa.PublicKey), []byte(message))
	if err != nil {
		log.Fatalf("failed to encrypt message: %v", err)
	}

	// Send the message to the receiver.
	resp, err := c.BifrostServiceClient.Send(ctx, &proto.SendRequest{
		Msg: &proto.Message{
			Timestamp: &timestamppb.Timestamp{
				Seconds: time.Now().Unix(),
			},
			Sender: c.username,
			Text:   string(cipherText),
		},
		Receiver: receiver,
	})
	if err != nil {
		log.Fatalf("error calling function Send: %v", err)
	}
	if resp.Response != "OK!" {
		log.Fatalf("failed to send message: %s", r.Response)
	}
}

func (c clientObj) Fetch() {
	stream, err := c.BifrostServiceClient.Fetch(context.Background(), &proto.FetchRequest{
		Username: c.username,
	})
	if err != nil {
		log.Fatalf("error calling function Fetch: %v", err)
	}
	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			return
		} else if err == nil && resp.Response == "OK!" {
			message := resp.Message
			// Decrypt the message with the user's private key.
			plainText, err := rsa.DecryptPKCS1v15(rand.Reader, c.privateKey, []byte(message.Text))
			if err != nil {
				log.Fatalf("failed to decrypt message: %v", err)
			}
			log.Printf("Message from %s at %s: %s", message.Sender, message.Timestamp.AsTime().String(), string(plainText))
		} else {
			log.Fatalf("failed to fetch message: %s", resp.Response)
		}
	}
}
