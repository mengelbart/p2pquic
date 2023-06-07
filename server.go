package p2pquic

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"io"
	"log"
	"math/big"
	"net"

	"github.com/quic-go/quic-go"
)

type Server struct {
}

func NewServer() (*Server, error) {
	s := &Server{}
	return s, nil
}

func (s *Server) RunICE(localPort, remotePort int) error {
	ice, err := NewICESignalingServer(localPort, remotePort)
	if err != nil {
		return err
	}
	err = ice.setup()
	if err != nil {
		return err
	}
	conn, err := ice.run(true)
	if err != nil {
		return err
	}
	log.Printf("got conn: %v\n", conn)

	return s.runWithConn(conn)
}

func (s *Server) RunUDP(addr string) error {
	laddr, err := net.ResolveUDPAddr("udp4", addr)
	if err != nil {
		return err
	}
	conn, err := net.ListenUDP("udp4", laddr)
	if err != nil {
		return err
	}
	return s.runWithConn(conn)
}

// Start a server that echos all data on the first stream opened by the client
func (s *Server) runWithConn(conn net.PacketConn) error {
	tr := quic.Transport{
		Conn: conn,
	}
	listener, err := tr.Listen(generateTLSConfig(), &quic.Config{})
	if err != nil {
		return err
	}

	quicConn, err := listener.Accept(context.Background())
	if err != nil {
		return err
	}
	stream, err := quicConn.AcceptStream(context.Background())
	if err != nil {
		return err
	}
	// Echo through the loggingWriter
	_, err = io.Copy(loggingWriter{stream}, stream)
	return err
}

// Setup a bare-bones TLS config for the server
func generateTLSConfig() *tls.Config {
	key, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		panic(err)
	}
	template := x509.Certificate{SerialNumber: big.NewInt(1)}
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		panic(err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	tlsCert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		panic(err)
	}
	return &tls.Config{
		Certificates: []tls.Certificate{tlsCert},
		NextProtos:   []string{"quic-echo-example"},
	}
}
