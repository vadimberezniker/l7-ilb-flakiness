package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/google"

	dpb "github.com/vadimberezniker/l7-ilb-flakiness/proto/dummy"
)

var (
	server      = flag.String("server", "", "")
	mode        = flag.String("mode", "", "")
	concurrency = flag.Int("concurrency", 10, "")
	delay = flag.Duration("delay", 1 * time.Second, "")
)

func ping(ctx context.Context, server string) error {
	eg, ctx := errgroup.WithContext(ctx)
	for i := 0; i < *concurrency; i++ {
		eg.Go(func() error {
			// Note that we intentionally dial for each goroutine to ensure we create a separate HTTP/2 connection for
			// each goroutine, otherwise the pings can get multiplexed into a single HTTP/2 connection.
			conn, err := grpc.Dial(server, grpc.WithTransportCredentials(google.NewDefaultCredentials().TransportCredentials()))
			if err != nil {
				return err
			}
			client := dpb.NewDummyClient(conn)
			time.Sleep(time.Duration(rand.Float64() * float64(*delay)))
			for {
				if _, err := client.Ping(ctx, &dpb.PingRequest{}); err != nil {
					log.Printf("error calling /Ping: %s", err)
				}
				time.Sleep(*delay)
			}
		})
	}

	return eg.Wait()
}

type regMonitor struct {
	mu sync.Mutex
	lastDisconnect          time.Time
	timesBetweenDisconnects []time.Duration
}

func (m *regMonitor) singleRegistrationStream(ctx context.Context, server string, testStart time.Time, id string) error {
	// Note that we intentionally dial for each registration to ensure we create a separate HTTP/2 connection for each
	// registration, otherwise the registrations will get multiplexed into a single HTTP/2 connection.
	conn, err := grpc.Dial(server, grpc.WithTransportCredentials(google.NewDefaultCredentials().TransportCredentials()))
	if err != nil {
		return err
	}
	client := dpb.NewDummyClient(conn)

	stream, err := client.Register(ctx)
	if err != nil {
		return err
	}

	registration := &dpb.RegisterRequest{Id: id}
	if err := stream.Send(registration); err != nil {
		log.Printf("could not send initial registration: %s", err)
		return err
	}
	log.Printf("[%s] sent initial registration", id)

	start := time.Now()

	done := make(chan struct{})
	defer close(done)

	go func() {
		for {
			select {
			case <-done:
				return
			case <-time.After(5 * time.Second):
				if err := stream.Send(registration); err != nil {
					log.Printf("[%s] could not send reg request: %s", id, err)
				}
			}
		}
	}()

	for {
		_, err := stream.Recv()
		if err != nil {
			log.Printf("[%s] reg err after %s (%fm from test start): %s", id, time.Since(start), time.Since(testStart).Minutes(), err)
			m.mu.Lock()
			// Coalesce disconnects that happen in a short period of time. They were probably terminated by the same
			// Envoy.
			timeSinceLastDisconnect := time.Since(m.lastDisconnect)
			if timeSinceLastDisconnect > 5 * time.Second {
				if !m.lastDisconnect.IsZero() {
					m.timesBetweenDisconnects = append(m.timesBetweenDisconnects, timeSinceLastDisconnect)
					total := 0.0
					for _, t := range m.timesBetweenDisconnects {
						total += t.Minutes()
					}
					avg := total / float64(len(m.timesBetweenDisconnects))
					log.Printf("Average time between disconnects: %fm", avg)
				}
				m.lastDisconnect = time.Now()
			}
			m.mu.Unlock()
			return err
		}
	}
}

func register(ctx context.Context, server string) error {
	testStart := time.Now()
	rm := &regMonitor{}
	for i := 0; i < *concurrency; i++ {
		id := fmt.Sprintf("%d", i)
		go func() {
			for {
				_ = rm.singleRegistrationStream(ctx, server, testStart, id)
				time.Sleep(1 * time.Second)
			}
		}()
	}
	select {}
}

func main() {
	flag.Parse()

	if *server == "" {
		log.Fatalf("--server is required")
	}
	if *mode == "" {
		log.Fatalf("--mode is required")
	}

	ctx := context.Background()

	switch *mode {
	case "register":
		if err := register(ctx, *server); err != nil {
			log.Fatalf("register error: %s", err)
		}
	case "ping":
		if err := ping(ctx, *server); err != nil {
			log.Fatalf("ping error: %s", err)
		}
	default:
		log.Fatalf("unknown --mode: %q", *mode)
	}
}
