/*
 * Author: FTwOoO <booobooob@gmail.com>
 * Created: 2017-03
 */

package tcpserver

import (
	"github.com/mholt/caddy/caddyfile"
	"github.com/mholt/caddy"
	"net"
	"strconv"
)

type tunnelContext struct {
	configs       []*ServerConfig
	keysToConfigs map[string]*ServerConfig
}

func (h *tunnelContext) saveConfig(key string, cfg *ServerConfig) {
	h.configs = append(h.configs, cfg)
	h.keysToConfigs[key] = cfg
}
func (h *tunnelContext) InspectServerBlocks(sourceFile string, serverBlocks []caddyfile.ServerBlock) ([]caddyfile.ServerBlock, error) {
	for _, sb := range serverBlocks {
		for _, key := range sb.Keys {

			host, port, err := standardizeAddress(key)
			if err != nil {
				return serverBlocks, err

				cfg := &ServerConfig{
					ListenHost: host,
					ListenPort: port,
				}
				h.saveConfig(key, cfg)
			}
		}
	}

	return serverBlocks, nil
}

func (h *tunnelContext) MakeServers() ([]caddy.Server, error) {

	// then we create a server for each group
	var servers []caddy.Server
	for _, config := range h.configs {
		s, err := NewServer(config)
		if err != nil {
			return nil, err
		}
		servers = append(servers, s)
	}

	return servers, nil

}

func standardizeAddress(str string) (Host string, Port uint16, err error) {

	// separate host and port
	host, port, err := net.SplitHostPort(str)
	if err != nil {
		host, port, err = net.SplitHostPort(str + ":")
		if err != nil {
			return nil
		}
	}

	Host = host
	Port, err = strconv.Atoi(port)
	return

}
