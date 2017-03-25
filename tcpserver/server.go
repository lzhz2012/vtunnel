/*
 * Author: FTwOoO <booobooob@gmail.com>
 * Created: 2017-03
 */

package tcpserver

import (
	"github.com/mholt/caddy"
	"sync"
	"time"
	"net"
	"runtime"
	"errors"
	"fmt"
)

var DefaultPort = 10010
var ServerType = "tunnel"

func init() {

	caddy.RegisterServerType(ServerType, caddy.ServerType{
		Directives: func() []string {
			return directives
		},
		DefaultInput: func() caddy.Input {
			return caddy.CaddyfileInput{
				Contents:       []byte(fmt.Sprintf("0.0.0.0:%d {clients 12345678}", DefaultPort)),
				ServerTypeName: ServerType,
			}
		},
		NewContext: func() caddy.Context {
			return new(tunnelContext)
		},
	})
}


// Server is the HTTP server implementation.
type Server struct {
	listener    net.Listener
	listenerMu  sync.Mutex
	config      *ServerConfig

	connTimeout time.Duration // max time to wait for a connection before force stop

	doneChan    chan struct{}
}

// ensure it satisfies the interface
var _ caddy.GracefulServer = new(Server)

var GracefulTimeout = 5 * time.Second
var ErrServerClosed = errors.New("http: Server closed")

// NewServer creates a new Server instance that will listen on addr
// and will serve the sites configured in group.
func NewServer(config *ServerConfig) (*Server, error) {
	s := &Server{
		config:       config,
		connTimeout: GracefulTimeout,
		doneChan: make(chan struct{}),
	}

	return s, nil
}


// Listen creates an active listener for s that can be
// used to serve requests.
func (s *Server) Listen() (net.Listener, error) {

	ln, err := net.Listen("tcp", s.config.ListenAddr)
	if err != nil {
		var succeeded bool
		if runtime.GOOS == "windows" {
			// Windows has been known to keep sockets open even after closing the listeners.
			// Tests reveal this error case easily because they call Start() then Stop()
			// in succession. TODO: Better way to handle this? And why limit this to Windows?
			for i := 0; i < 20; i++ {
				time.Sleep(100 * time.Millisecond)
				ln, err = net.Listen("tcp", s.config.ListenAddr)
				if err == nil {
					succeeded = true
					break
				}
			}
		}
		if !succeeded {
			return nil, err
		}
	}

	if tcpLn, ok := ln.(*net.TCPListener); ok {
		ln = tcpKeepAliveListener{TCPListener: tcpLn}
	}

	cln := ln.(caddy.Listener)

	// Very important to return a concrete caddy.Listener
	// implementation for graceful restarts.
	return cln.(caddy.Listener), nil
}


// Serve serves requests on ln. It blocks until ln is closed.
func (s *Server) Serve(ln net.Listener) error {
	s.listenerMu.Lock()
	s.listener = ln
	s.listenerMu.Unlock()

	var err error

	handler := s.config.GetHandler()
	if handler == nil {
		return errors.New("Invalid config")
	}
	err = handler(ln)
	return err
}

func (s *Server) ListenPacket() (net.PacketConn, error) {
	return nil, nil
}

func (s *Server) ServePacket(pc net.PacketConn) error {
	return nil
}

// Address returns the address s was assigned to listen on.
func (s *Server) Address() string {
	return s.config.ListenAddr
}

// Stop stops s gracefully (or forcefully after timeout) and
// closes its listener.
func (s *Server) Stop() error {
	close(s.doneChan)
	return nil
}

