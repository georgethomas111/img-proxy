package main

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/georgethomas111/img-proxy/learn"
	"google.golang.org/grpc"
)

type client struct {
	stream     learn.Learn_GossipClient
	grpcConn   *grpc.ClientConn
	grpcAddr   string
	cliName    string
	Updates    chan string
	Errors     chan error
	KillClient chan int
}

func New(grpcAddr string, cliName string) *client {
	c := &client{
		grpcAddr:   grpcAddr,
		cliName:    cliName,
		Updates:    make(chan string),
		Errors:     make(chan error),
		KillClient: make(chan int),
	}

	ctx, cancel := context.WithCancel(context.Background())

	// This is to ensure that there is a way to
	// cancel the client without leaks.
	go func() {
		for {
			<-c.KillClient
			cancel()
			c.stream.CloseSend()
			return
		}

	}()

	go c.recvUpdates(ctx)
	return c
}

func (c *client) recvOneUpdate() (string, error) {
	if c.stream == nil {
		return "", errors.New("stream not initialized " +
			c.cliName + " " + c.grpcAddr)
	}

	update, err := c.stream.Recv()

	if update != nil {
		return update.Info, err
	}

	return "", err
}

func (c *client) recvUpdates(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			update, err := c.recvOneUpdate()
			if err != nil {
				c.Errors <- err
				c.ensureStream(ctx)
				continue
			}
			c.Updates <- update
		}
	}
}

func (c *client) ensureStream(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			if c.grpcConn != nil {
				c.grpcConn.Close()
			}

			s, err := c.Connect(ctx)
			if err != nil {
				c.Errors <- err
				fmt.Println("Sleeping for a bit and trying to connect again")
				// A second of wait time so as to not flood the server.
				time.Sleep(time.Second)
				continue
			}
			c.stream = s
			fmt.Println("got the stream now breaking")
			return
		}
	}
}

func (c *client) Connect(ctx context.Context) (learn.Learn_GossipClient, error) {
	fmt.Println("Trying to connect to server")
	conn, err := grpc.Dial(c.grpcAddr, grpc.WithInsecure())
	if err != nil {
		return nil, errors.New("Error connecting to grpc server " + err.Error())
	}

	c.grpcConn = conn

	req := &learn.GossipRequest{
		Id: c.cliName,
	}
	grpcCli := learn.NewLearnClient(conn)

	stream, err := grpcCli.Gossip(ctx, req)
	if err != nil {
		return nil, errors.New("Error gossiping " + err.Error())
	}
	return stream, nil
}

func main() {
	grpcAddr := "0.0.0.0:8080"
	cliName := "cli-abc"

	cli := New(grpcAddr, cliName)

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case update := <-cli.Updates:
				fmt.Println("Update from server: ", update)

			case err := <-cli.Errors:
				fmt.Println("Errors from server: ", err.Error())
			}
		}
	}()

	fmt.Println("Client started")
	wg.Wait()
}
