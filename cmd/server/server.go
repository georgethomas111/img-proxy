package main

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/georgethomas111/img-proxy/learn"
	"google.golang.org/grpc"
)

type StreamingServer struct {
	learn.UnimplementedLearnServer // Embed the unimplemented methods
	streams                        map[string]chan string
}

func (s *StreamingServer) Gossip(req *learn.GossipRequest, stream learn.Learn_GossipServer) error {
	if req == nil {
		return errors.New("Request cannot be nil")
	}

	s.streams[req.Id] = make(chan string)

	ctx := stream.Context()
	for {
		select {
		case <-ctx.Done():
			fmt.Println("Received done from client, exiting")
			delete(s.streams, req.Id)
			return nil
		case info := <-s.streams[req.Id]:
			msg := &learn.Messages{
				Info: info,
			}
			err := stream.Send(msg)
			if err != nil {
				fmt.Println("Error streaming message from "+
					"server to client with id ", req.Id, err.Error())
			}
		}
	}

	return nil
}

// Update time updates the current time to all the clients.
func (s *StreamingServer) UpdateTime() {
	timeString := strconv.Itoa(int(time.Now().Unix()))

	for id, stream := range s.streams {
		fmt.Println("Trying to write message to stream ", id, timeString)
		stream <- timeString
	}
}

func (s *StreamingServer) Start(grpcAddr string) error {
	lis, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		return err
	}

	g := grpc.NewServer()
	learn.RegisterLearnServer(g, s)

	fmt.Println("Listening for grpc requests on ", grpcAddr)
	return g.Serve(lis)
}

func New() *StreamingServer {
	return &StreamingServer{
		streams: make(map[string]chan string),
	}
}

func main() {
	grpcAddr := "0.0.0.0:8080"
	s := New()
	go func() {
		for {
			select {
			case <-time.After(time.Second):
				s.UpdateTime()
			}
		}
	}()
	err := s.Start(grpcAddr)
	if err != nil {
		fmt.Println("Error starting server ", err.Error())
	}
}
