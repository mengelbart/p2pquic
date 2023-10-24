package p2pquic

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strings"

	"github.com/pion/ice/v2"
)

type ICESignalingServer struct {
	agent         *ice.Agent
	candidateChan chan ice.Candidate
	remoteChan    chan string
	remotePort    int
	localPort     int
}

func NewICESignalingServer(remotePort, localPort int) (*ICESignalingServer, error) {
	iceAgent, err := ice.NewAgent(&ice.AgentConfig{
		NetworkTypes: []ice.NetworkType{ice.NetworkTypeUDP4},
	})
	if err != nil {
		return nil, err
	}
	return &ICESignalingServer{
		agent:         iceAgent,
		candidateChan: make(chan ice.Candidate),
		remoteChan:    make(chan string, 3),
		remotePort:    remotePort,
		localPort:     localPort,
	}, nil
}

func (s *ICESignalingServer) setup() error {
	if err := s.agent.OnCandidate(func(c ice.Candidate) {
		s.candidateChan <- c
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

type Signal struct {
	Ufrag      string   `json:"ufrag"`
	Pwd        string   `json:"pwd"`
	Candidates []string `json:"candidates"`
}

func (s *ICESignalingServer) run(controlling bool) (net.PacketConn, error) {
	// Get the local auth details and send to remote peer
	localUfrag, localPwd, err := s.agent.GetLocalUserCredentials()
	if err != nil {
		return nil, err
	}

	s.agent.GatherCandidates()

	var cs []string
	for c := range s.candidateChan {
		if c == nil {
			break
		}
		cs = append(cs, c.Marshal())
	}
	sig := Signal{
		Ufrag:      localUfrag,
		Pwd:        localPwd,
		Candidates: cs,
	}
	fmt.Println("local signal:")
	fmt.Println(Encode(sig))
	log.Printf("waiting for remote signal")

	var remoteSignal Signal
	Decode(MustReadStdin(), &remoteSignal)

	log.Printf("got remote signal: %v", remoteSignal)

	for _, c := range remoteSignal.Candidates {
		var candidate ice.Candidate
		candidate, err = ice.UnmarshalCandidate(c)
		if err != nil {
			panic(err)
		}
		if err = s.agent.AddRemoteCandidate(candidate); err != nil {
			panic(err)
		}
	}

	fmt.Print("Press 'Enter' when both processes are ready")
	if _, err = bufio.NewReader(os.Stdin).ReadBytes('\n'); err != nil {
		return nil, err
	}
	var conn *ice.Conn
	if controlling {
		conn, err = s.agent.Accept(context.TODO(), remoteSignal.Ufrag, remoteSignal.Pwd)
	} else {
		conn, err = s.agent.Dial(context.TODO(), remoteSignal.Ufrag, remoteSignal.Pwd)
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

// Encode encodes the input in base64
// It can optionally zip the input before encoding
func Encode(obj interface{}) string {
	b, err := json.Marshal(obj)
	if err != nil {
		panic(err)
	}

	return base64.StdEncoding.EncodeToString(b)
}

// Decode decodes the input from base64
// It can optionally unzip the input after decoding
func Decode(in string, obj interface{}) {
	b, err := base64.StdEncoding.DecodeString(in)
	if err != nil {
		panic(err)
	}

	err = json.Unmarshal(b, obj)
	if err != nil {
		panic(err)
	}
}

// MustReadStdin blocks until input is received from stdin
func MustReadStdin() string {
	r := bufio.NewReader(os.Stdin)

	var in string
	for {
		var err error
		in, err = r.ReadString('\n')
		if err != io.EOF {
			if err != nil {
				panic(err)
			}
		}
		in = strings.TrimSpace(in)
		if len(in) > 0 {
			break
		}
	}

	fmt.Println("")

	return in
}
