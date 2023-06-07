package p2pquic

import (
	"context"
	"crypto/tls"
	"io"
	"log"
	"net"
	"time"

	"github.com/quic-go/quic-go"
)

type Client struct {
	message string
}

func NewClient(message string) (*Client, error) {
	return &Client{
		message: message,
	}, nil
}

func (c *Client) RunICE(localPort, remotePort int) error {
	ice, err := NewICESignalingServer(localPort, remotePort)
	if err != nil {
		return err
	}
	err = ice.setup()
	if err != nil {
		return err
	}
	conn, err := ice.run(false)
	if err != nil {
		return err
	}
	log.Printf("got conn: %v\n", conn)

	return c.runWithConn(conn, conn.LocalAddr().String())
}

func (c *Client) RunUDP(addr string) error {
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4zero, Port: 0})
	if err != nil {
		return err
	}
	return c.runWithConn(conn, addr)
}

func (c *Client) runWithConn(conn net.PacketConn, addr string) error {
	tlsConf := &tls.Config{
		InsecureSkipVerify: true,
		NextProtos:         []string{"quic-echo-example"},
	}
	raddr, err := net.ResolveUDPAddr("udp4", addr)
	if err != nil {
		return err
	}

	tr := quic.Transport{
		Conn: conn,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	quicConn, err := tr.Dial(ctx, raddr, tlsConf, &quic.Config{})
	if err != nil {
		return err
	}

	stream, err := quicConn.OpenStreamSync(context.Background())
	if err != nil {
		return err
	}

	log.Printf("Client: Sending '%s'\n", c.message)
	_, err = stream.Write([]byte(c.message))
	if err != nil {
		return err
	}

	buf := make([]byte, len(c.message))
	_, err = io.ReadFull(stream, buf)
	if err != nil {
		return err
	}
	log.Printf("Client: Got '%s'\n", buf)

	return nil
}
