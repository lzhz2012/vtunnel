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
var serverType = "tunnel"

var directives = []string{
	"server",
	"clients",
}

func init() {

	caddy.RegisterServerType(serverType, caddy.ServerType{
		Directives: func() []string {
			return directives
		},
		DefaultInput: func() caddy.Input {
			return caddy.CaddyfileInput{
				Contents:       []byte(fmt.Sprintf("0.0.0.0:%d {clients 12345678}", DefaultPort)),
				ServerTypeName: serverType,
			}
		},
		NewContext: new(tunnelContext),
	})

	caddy.RegisterPlugin(serverType, caddy.Plugin{
		ServerType: "tunnel",
		Action:     SetupTunnelPlugin,
	})


	//caddy.RegisterCaddyfileLoader("short", caddy.LoaderFunc(shortCaddyfileLoader))

}


// Server is the HTTP server implementation.
type Server struct {
	listener    net.Listener
	listenerMu  sync.Mutex
	Addr        string
	config      *Config
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
		Addr: fmt.Sprintf("%s:%d", config.ListenHost, config.ListenPort),
		config:       config,
		connTimeout: GracefulTimeout,
		doneChan: make(chan struct{}),
	}

	return s, nil
}


// Listen creates an active listener for s that can be
// used to serve requests.
func (s *Server) Listen() (net.Listener, error) {

	ln, err := net.Listen("tcp", s.Addr)
	if err != nil {
		var succeeded bool
		if runtime.GOOS == "windows" {
			// Windows has been known to keep sockets open even after closing the listeners.
			// Tests reveal this error case easily because they call Start() then Stop()
			// in succession. TODO: Better way to handle this? And why limit this to Windows?
			for i := 0; i < 20; i++ {
				time.Sleep(100 * time.Millisecond)
				ln, err = net.Listen("tcp", s.Addr)
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
	var tempDelay time.Duration // how long to sleep on accept failure

	for {
		rw, e := ln.Accept()
		if e != nil {
			select {
			case <-s.doneChan:
				return ErrServerClosed
			default:
			}
			if ne, ok := e.(net.Error); ok && ne.Temporary() {
				if tempDelay == 0 {
					tempDelay = 5 * time.Millisecond
				} else {
					tempDelay *= 2
				}
				if max := 1 * time.Second; tempDelay > max {
					tempDelay = max
				}
				//s.logf("http: Accept error: %v; retrying in %v", e, tempDelay)
				time.Sleep(tempDelay)
				continue
			}
			return e
		}
		tempDelay = 0

		//c := srv.newConn(rw)
		//c.setState(c.rwc, StateNew) // before Serve can return
		//go c.serve(ctx)
	}

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
	return s.Addr
}

// Stop stops s gracefully (or forcefully after timeout) and
// closes its listener.
func (s *Server) Stop() error {
	close(s.doneChan)
	return nil
}
