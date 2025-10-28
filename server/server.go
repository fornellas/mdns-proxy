package server

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/fornellas/slogxt/log"

	"github.com/fornellas/mdns-proxy/mdns"
)

type Config struct {
	// Addr optionally specifies the TCP address for the server to listen on,
	// in the form "host:port". If empty, ":http" (port 80) is used.
	// The service names are defined in RFC 6335 and assigned by IANA.
	// See net.Dial for details of the address format.
	ListenAddr string
	// Base domain where the proxy will be accessed
	BaseDomain string
	// mDNS multicast interface to use
	MdnsInterfaceName string
	// mDNS Service
	MdnsService string
	// mDNS Domain
	MdnsDomain string
	// mDNS browse timeout
	MdnsBrowseTimeout time.Duration
	// Whether to disable usage of IPv4 for mDNS operations. Does not affect discovered addresses.
	MdnsDisableIPv4 bool
	// Whether to disable usage of IPv6 for mDNS operations. Does not affect discovered addresses.
	MdnsDisableIPv6 bool
}

func (c *Config) MdnsProto() mdns.Proto {
	if c.MdnsDisableIPv4 {
		return mdns.ProtoInet6
	}
	if c.MdnsDisableIPv6 {
		return mdns.ProtoInet
	}
	return mdns.ProtoAny
}

func getScheme(req *http.Request) string {
	scheme := req.Header.Get("X-Scheme")
	if scheme == "" {
		scheme = "http"
		if req.TLS != nil {
			scheme = "https"
		}
	}
	return scheme
}

func getAddrPort(req *http.Request) (string, int, error) {
	var addr string
	var port int
	var err error
	addrPort := strings.Split(req.Host, ":")
	if len(addrPort) < 2 {
		addr = req.Host
		port = 80
		if getScheme(req) == "https" {
			port = 443
		}
	} else {
		portStr := addrPort[len(addrPort)-1]
		addr = strings.TrimSuffix(req.Host, fmt.Sprintf(":%s", portStr))
		port, err = strconv.Atoi(portStr)
		if err != nil {
			return "", 0, err
		}
	}
	return addr, port, nil
}

func handleListMdnsHosts(
	ctx context.Context,
	config *Config,
	w http.ResponseWriter,
	req *http.Request,
) {
	ctx, logger := log.MustWithGroup(ctx, "List MDNS Hosts")
	defer logger.Debug("Finished")

	m, err := mdns.NewMDNS()
	if err != nil {
		msg := fmt.Sprintf("NewMDNS() failed: %v", err)
		logger.Error(msg)
		http.Error(w, msg, http.StatusInternalServerError)
		return
	}
	defer func() {
		logger.Debug("Closing MDNS")
		m.Close()
		logger.Debug("Closed MDNS")
	}()

	scheme := getScheme(req)

	_, port, err := getAddrPort(req)
	if err != nil {
		msg := fmt.Sprintf("Failed get port: Error identifying host address and port '%s': %v", req.Host, err)
		logger.Error(msg)
		http.Error(w, msg, http.StatusInternalServerError)
		return
	}

	logger.Info("Browsing mDNS services")
	services, err := m.BrowseServices(
		ctx,
		config.MdnsInterfaceName,
		config.MdnsProto(),
		config.MdnsService,
		config.MdnsDomain,
		config.MdnsBrowseTimeout,
	)
	if err != nil {
		msg := fmt.Sprintf("Error querying mDNS: %v", err)
		logger.Error(msg)
		http.Error(w, msg, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html")

	fmt.Fprint(w, `
			<!DOCTYPE html>
				<html>
				<head>
					<title>mDNS Hosts</title>
				</head>
				<body>
					<h1>mDNS Hosts</h1>
					<ul>
		`)

	hosts := []string{}
	for _, service := range services {
		if service.Port != 80 {
			continue
		}
		hosts = append(hosts, service.Host)
	}
	sort.Strings(hosts)

	var last_host string
	for _, host := range hosts {
		if host == last_host {
			continue
		}
		fmt.Fprintf(w, `					<li><a href="%s://%s.%s:%d/">%s</a></li>`,
			scheme,
			strings.TrimSuffix(host, fmt.Sprintf(".%s", config.MdnsDomain)),
			config.BaseDomain,
			port,
			host,
		)
		last_host = host
	}

	fmt.Fprint(w, `
			</ul>
		</body>
		</html>
	`)
}

func handleProxyMdnsHosts(
	ctx context.Context,
	config *Config,
	w http.ResponseWriter,
	req *http.Request,
) {
	ctx, logger := log.MustWithGroup(ctx, "Proxy mDNS Host Request")
	defer logger.Debug("Finished")

	m, err := mdns.NewMDNS()
	if err != nil {
		msg := fmt.Sprintf("NewMDNS() failed: %v", err)
		logger.Error(msg)
		http.Error(w, msg, http.StatusInternalServerError)
		return
	}
	defer func() {
		logger.Debug("Closing MDNS")
		m.Close()
		logger.Debug("Closed MDNS")
	}()

	addr, _, err := getAddrPort(req)
	if err != nil {
		msg := fmt.Sprintf("Error identifying host address and port '%s': %v", req.Host, err)
		logger.Error(msg)
		http.Error(w, msg, http.StatusInternalServerError)
		return
	}

	host := fmt.Sprintf("%s.%s", strings.TrimSuffix(addr, fmt.Sprintf(".%s", config.BaseDomain)), config.MdnsDomain)

	logger.Info("Resolving mDNS host")
	ip, err := m.ResolveHost(ctx, host, config.MdnsInterfaceName, config.MdnsProto())
	if err != nil {
		msg := fmt.Sprintf("Error resolving mDNS host '%s': %v", host, err)
		logger.Error(msg)
		http.Error(w, msg, http.StatusInternalServerError)
		return
	}

	req.URL.Scheme = "http"
	req.URL.User = nil
	req.URL.Host = host
	req.Header["Host"] = []string{host}
	req.Host = host

	_, logger = log.MustWithGroupAttrs(
		ctx,
		"HTTP Request",
		"Method", req.Method,
		"Proto", req.Proto,
		"Host", req.Host,
		"RequestURI", req.RequestURI,
	)
	logger.Info("Proxying request")

	httputil.NewSingleHostReverseProxy(&url.URL{
		Scheme: "http",
		Host:   ip.String(),
	}).ServeHTTP(
		w, req,
	)
}

func getRootRouter(config *Config) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, req *http.Request) {
		ctx, logger := log.MustWithGroupAttrs(
			req.Context(),
			"HTTP Request",
			"Method", req.Method,
			"Proto", req.Proto,
			"Host", req.Host,
			"RequestURI", req.RequestURI,
		)

		hostSlice := strings.Split(req.Host, ":")
		host := hostSlice[0]
		if host == config.BaseDomain {
			if req.URL.Path != "/" {
				msg := "Not found"
				logger.Error(msg)
				http.Error(w, msg, http.StatusNotFound)
				return
			}
			handleListMdnsHosts(ctx, config, w, req)
			return
		}

		if strings.HasSuffix(host, fmt.Sprintf(".%s", config.BaseDomain)) {
			hostSlice := strings.Split(host, fmt.Sprintf(".%s", config.BaseDomain))
			if len(hostSlice) != 2 {
				msg := fmt.Sprintf("host must be in the format ${mdns_host}.%s, got: %s", config.BaseDomain, host)
				logger.Error("Bad request", "message", msg)
				http.Error(w, msg, http.StatusBadRequest)
				return
			}
			mdnsHost := hostSlice[0]
			if len(strings.Split(mdnsHost, ".")) != 1 {
				msg := fmt.Sprintf("host must be in the format ${mdns_host}.%s, got: %s", config.BaseDomain, host)
				logger.Error("Bad request", "message", msg)
				http.Error(w, msg, http.StatusBadRequest)
				return
			}
			mdnsHost = fmt.Sprintf("%s.%s", mdnsHost, config.MdnsDomain)
			req.Header["Host"] = []string{mdnsHost}
			handleProxyMdnsHosts(ctx, config, w, req)
			return
		}

		msg := fmt.Sprintf("Bad request: unexpected host: %s", req.Host)
		logger.Error("Bad request", "message", msg)
		http.Error(w, msg, http.StatusBadRequest)
	}
}

func NewServer(
	ctx context.Context,
	config Config,
) (
	http.Server,
	error,
) {
	serveMux := http.NewServeMux()
	serveMux.HandleFunc("/", getRootRouter(&config))

	return http.Server{
		Addr:    config.ListenAddr,
		Handler: serveMux,
		// TODO
		// ReadTimeout time.Duration
		// ReadHeaderTimeout time.Duration
		// WriteTimeout time.Duration
		// IdleTimeout time.Duration
		BaseContext: func(l net.Listener) context.Context {
			ctx, _ := log.MustWithGroupAttrs(
				ctx,
				"HTTP Server",
				"listen-addr", config.ListenAddr,
				"base-domain", config.BaseDomain,
				"mdns-interface-name", config.MdnsInterfaceName,
				"mdns-service", config.MdnsService,
				"mdns-domain", config.MdnsDomain,
				"mdns-browse-timeout", config.MdnsBrowseTimeout,
				"mdns-disable-ipv4", config.MdnsDisableIPv4,
				"mdns-disable-ipv6", config.MdnsDisableIPv6,
			)
			return ctx
		},
		ConnContext: func(ctx context.Context, c net.Conn) context.Context {
			ctx, _ = log.MustWithGroupAttrs(
				ctx,
				"Connection",
				"LocalAddr", c.LocalAddr(),
				"RemoteAddr", c.RemoteAddr(),
			)
			return ctx
		},
	}, nil
}
