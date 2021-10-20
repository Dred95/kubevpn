package pkg

import (
	"context"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/wencaiwulue/kubevpn/core"
	"github.com/wencaiwulue/kubevpn/tun"
	"net"
	"strings"
)

type Route struct {
	ServeNodes []string // -L tun
	ChainNodes string   // -F socks5
	Retries    int
}

func (r *Route) parseChain() (*core.Chain, error) {
	chain := core.NewChain()
	chain.Retries = r.Retries

	// parse the base nodes
	nodes, err := parseChainNode(r.ChainNodes)
	if err != nil {
		return nil, err
	}

	chain.AddNodeGroup(nodes)

	return chain, nil
}

func parseChainNode(ns string) (*core.Node, error) {
	node, err := core.ParseNode(ns)
	if err != nil {
		return nil, err
	}
	node.Client = &core.Client{
		Connector:   core.SOCKS5UDPTunConnector(),
		Transporter: core.TCPTransporter(),
	}
	return &node, nil
}

func (r *Route) GenRouters() ([]router, error) {
	chain, err := r.parseChain()
	if err != nil {
		if !errors.Is(err, core.ErrInvalidNode) {
			return nil, err
		}
	}

	routers := make([]router, 0, len(r.ServeNodes))
	for _, serveNode := range r.ServeNodes {
		node, err := core.ParseNode(serveNode)
		if err != nil {
			return nil, err
		}

		tunRoutes := parseIPRoutes(node.Get("route"))
		gw := net.ParseIP(node.Get("gw")) // default gateway
		for i := range tunRoutes {
			if tunRoutes[i].Gateway == nil {
				tunRoutes[i].Gateway = gw
			}
		}

		var ln tun.Listener
		switch node.Transport {
		case "tcp":
			ln, err = core.TCPListener(node.Addr)
		case "tun":
			cfg := tun.TunConfig{
				Name:    node.Get("name"),
				Addr:    node.Get("net"),
				Peer:    node.Get("peer"),
				MTU:     node.GetInt("mtu"),
				Routes:  tunRoutes,
				Gateway: node.Get("gw"),
			}
			ln, err = tun.TunListener(cfg)
		default:
			ln, err = core.TCPListener(node.Addr)
		}
		if err != nil {
			return nil, err
		}

		var handler core.Handler
		switch node.Protocol {
		case "tun":
			handler = core.TunHandler()
		default:
			handler = core.SOCKS5Handler()
		}

		handler.Init(
			core.ChainHandlerOption(chain),
			core.NodeHandlerOption(node),
			core.IPRoutesHandlerOption(tunRoutes...),
		)

		rt := router{
			node:    node,
			server:  &core.Server{Listener: ln},
			handler: handler,
			chain:   chain,
		}
		routers = append(routers, rt)
	}

	return routers, nil
}

type router struct {
	node    core.Node
	server  *core.Server
	handler core.Handler
	chain   *core.Chain
}

func (r *router) Serve(ctx context.Context) error {
	log.Debugf("%s on %s", r.node.String(), r.server.Addr())
	return r.server.Serve(ctx, r.handler)
}

func (r *router) Close() error {
	if r == nil || r.server == nil {
		return nil
	}
	return r.server.Close()
}

func parseIPRoutes(routeStringList string) (routes []tun.IPRoute) {
	if len(routeStringList) == 0 {
		return
	}

	routeList := strings.Split(routeStringList, ",")
	for _, route := range routeList {
		if _, ipNet, _ := net.ParseCIDR(strings.TrimSpace(route)); ipNet != nil {
			routes = append(routes, tun.IPRoute{Dest: ipNet})
		}
	}
	return
}
