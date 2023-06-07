package p2pquic

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"

	"github.com/pion/ice/v2"
)

type ICESignalingServer struct {
	agent      *ice.Agent
	remoteChan chan string
	remotePort int
	localPort  int
}

func NewICESignalingServer(remotePort, localPort int) (*ICESignalingServer, error) {
	iceAgent, err := ice.NewAgent(&ice.AgentConfig{
		NetworkTypes: []ice.NetworkType{ice.NetworkTypeUDP4},
	})
	if err != nil {
		return nil, err
	}
	return &ICESignalingServer{
		agent:      iceAgent,
		remoteChan: make(chan string, 3),
		remotePort: remotePort,
		localPort:  localPort,
	}, nil
}

func (s *ICESignalingServer) setup() error {
	http.HandleFunc("/remoteAuth", s.remoteAuthHandler(s.remoteChan))
	http.HandleFunc("/remoteCandidate", s.remoteCandidateHandler(s.agent))

	go func() {
		if err := http.ListenAndServe(fmt.Sprintf(":%d", s.localPort), nil); err != nil {
			panic(err)
		}
	}()

	if err := s.agent.OnCandidate(func(c ice.Candidate) {
		if c == nil {
			return
		}
		log.Printf("got candidate: %v\n", c.String())
		_, err := http.PostForm(fmt.Sprintf("http://localhost:%d/remoteCandidate", s.remotePort), //nolint
			url.Values{
				"candidate": {c.Marshal()},
			})
		if err != nil {
			panic(err)
		}
	}); err != nil {
		return err
	}
	// When ICE Connection state has change print to stdout
	if err := s.agent.OnConnectionStateChange(func(c ice.ConnectionState) {
		log.Printf("ICE Connection State has changed: %s\n", c.String())
	}); err != nil {
		return err
	}
	return nil
}

func (s *ICESignalingServer) run(controlling bool) (net.PacketConn, error) {
	fmt.Print("Press 'Enter' when both processes have started")
	if _, err := bufio.NewReader(os.Stdin).ReadBytes('\n'); err != nil {
		return nil, err
	}
	// Get the local auth details and send to remote peer
	localUfrag, localPwd, err := s.agent.GetLocalUserCredentials()
	if err != nil {
		return nil, err
	}

	_, err = http.PostForm(fmt.Sprintf("http://localhost:%d/remoteAuth", s.remotePort), //nolint
		url.Values{
			"ufrag": {localUfrag},
			"pwd":   {localPwd},
		})
	if err != nil {
		return nil, err
	}

	log.Printf("waiting for remote")

	remoteUfrag := <-s.remoteChan
	remotePwd := <-s.remoteChan

	s.agent.GatherCandidates()

	var conn *ice.Conn
	if controlling {
		conn, err = s.agent.Accept(context.TODO(), remoteUfrag, remotePwd)
	} else {
		conn, err = s.agent.Dial(context.TODO(), remoteUfrag, remotePwd)
	}
	if err != nil {
		return nil, err
	}

	pair, err := s.agent.GetSelectedCandidatePair()
	if err != nil {
		return nil, err
	}
	raddr, err := net.ResolveUDPAddr("udp4", fmt.Sprintf("%v:%v", pair.Remote.Address(), pair.Remote.Port()))
	if err != nil {
		return nil, err
	}

	log.Printf("remote addr: %v\n", raddr)

	pConn := &ICEPacketConn{
		Conn: conn,
		addr: raddr,
	}
	return pConn, nil
}

type ICEPacketConn struct {
	*ice.Conn
	addr net.Addr
}

// ReadFrom implements net.PacketConn
func (c *ICEPacketConn) ReadFrom(p []byte) (int, net.Addr, error) {
	n, err := c.Read(p)
	return n, c.addr, err
}

// WriteTo implements net.PacketConn
func (c *ICEPacketConn) WriteTo(p []byte, addr net.Addr) (int, error) {
	return c.Write(p)
}

func (s *ICESignalingServer) remoteCandidateHandler(iceAgent *ice.Agent) http.HandlerFunc {
	return func(_ http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			panic(err)
		}

		c, err := ice.UnmarshalCandidate(r.PostForm["candidate"][0])
		if err != nil {
			panic(err)
		}

		if err := iceAgent.AddRemoteCandidate(c); err != nil { //nolint:contextcheck
			panic(err)
		}
	}
}

// HTTP Listener to get ICE Credentials from remote Peer
func (s *ICESignalingServer) remoteAuthHandler(remoteAuthChannel chan<- string) http.HandlerFunc {
	return func(_ http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			panic(err)
		}

		remoteAuthChannel <- r.PostForm["ufrag"][0]
		remoteAuthChannel <- r.PostForm["pwd"][0]
	}
}
