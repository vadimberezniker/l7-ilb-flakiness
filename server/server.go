package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"log"
	"math/big"
	"net"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"

	dpb "github.com/vadimberezniker/l7-ilb-flakiness/proto/dummy"
)

type rpcPeer struct {
	lastSeen time.Time
}

type server struct {
	mu    sync.Mutex
	peers map[string]*rpcPeer
}

func (s *server) Register(stream dpb.Dummy_RegisterServer) error {
	sPeer, ok := peer.FromContext(stream.Context())
	if !ok {
		return status.Error(codes.Unavailable, "could not extract peer info")
	}

	id := "unknown"
	initial := true
	for {
		msg, err := stream.Recv()
		if err != nil {
			log.Printf("Stream ended from peer %q, id: %s: %s", sPeer.Addr, id, err)
			return err
		}
		if initial {
			initial = false
			id = msg.Id
			log.Printf("New Register stream from peer %q, id %s", sPeer.Addr, id)
		}
	}
}

func (s *server) Ping(ctx context.Context, request *dpb.PingRequest) (*dpb.PingResponse, error) {
	sPeer, ok := peer.FromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unavailable, "could not extract peer info")
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.peers[sPeer.Addr.String()]
	if !ok {
		p = &rpcPeer{}
		s.peers[sPeer.Addr.String()] = p
	}
	p.lastSeen = time.Now()

	return &dpb.PingResponse{}, nil
}

func selfSignedTLSConfig() (*tls.Config, error) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, err
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"Bug Repro Incorporated"},
		},
		NotBefore: time.Now(),
		NotAfter:  time.Now().Add(30 * 24 * time.Hour),

		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
	}
	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return nil, err
	}
	certBytes := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	privBytes, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return nil, err
	}
	keyBytes := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privBytes})

	pair, err := tls.X509KeyPair(certBytes, keyBytes)
	if err != nil {
		return nil, err
	}

	clientCACertPool := x509.NewCertPool()
	grpcTLSConfig := &tls.Config{
		NextProtos:               []string{"http/1.1"},
		MinVersion:               tls.VersionTLS12,
		SessionTicketsDisabled:   true,
		PreferServerCipherSuites: true,
		ClientAuth:               tls.VerifyClientCertIfGiven,
		ClientCAs:                clientCACertPool,
		Certificates:             []tls.Certificate{pair},
	}
	return grpcTLSConfig, nil
}

func newServer() *server {
	s := &server{peers: map[string]*rpcPeer{}}
	go func() {
		for {
			time.Sleep(1 * time.Minute)
			peers := []string{}
			s.mu.Lock()
			for p, d := range s.peers {
				if time.Since(d.lastSeen) < time.Minute {
					peers = append(peers, p)
				}
			}
			s.mu.Unlock()
			sort.Strings(peers)
			log.Printf("Peers seen in the last minute: \n%s", strings.Join(peers, "\n"))
		}
	}()
	return s
}

func main() {
	tlsConfig, err := selfSignedTLSConfig()
	if err != nil {
		log.Fatalf("Could not setup TLS config: %s", err)
	}

	gRPCListener, err := net.Listen("tcp", "0.0.0.0:1986")
	if err != nil {
		log.Fatal(err.Error())
	}

	s := newServer()
	grpcServer := grpc.NewServer(grpc.Creds(credentials.NewTLS(tlsConfig)))
	dpb.RegisterDummyServer(grpcServer, s)

	log.Printf("Server ready...")
	go grpcServer.Serve(gRPCListener)

	http.HandleFunc("/", func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write([]byte("ok"))
	})
	http.ListenAndServe("0.0.0.0:8080", nil)
}
