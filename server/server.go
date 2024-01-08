package server

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/fornellas/mdns-proxy/mdns"
)

func getScheme(req *http.Request) string {
	scheme := req.Header.Get("X-Forwarded-Proto")
	if scheme == "" {
		scheme = "http"
		if req.TLS != nil {
			scheme = "https"
		}
	}
	return scheme
}

func handleListMdnsHosts(
	listenPort int,
	baseDomain string,
	m mdns.MDNS,
	ifaceName string,
	service string,
	mdnsDomain string,
	timeout time.Duration,
	proto mdns.Proto,
	w http.ResponseWriter,
	req *http.Request,
) {
	scheme := getScheme(req)

	services, err := m.BrowseServices(
		ifaceName,
		proto,
		service,
		mdnsDomain,
		timeout,
	)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "Error querying mDNS: %v", err)
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

	hosts := map[string]bool{}
	for _, service := range services {
		hosts[service.Host] = true
	}

	for host := range hosts {
		fmt.Fprintf(w, `					<li><a href="%s://%s.%s:%d/">%s</a></li>`,
			scheme,
			strings.TrimSuffix(host, fmt.Sprintf(".%s", mdnsDomain)),
			baseDomain,
			listenPort,
			host,
		)
	}

	fmt.Fprint(w, `
			</ul>
		</body>
		</html>
	
	`)
}

func getAddrPort(req *http.Request) (string, int, error) {
	var addr string
	var port int
	var err error
	addrPort := strings.Split(req.Host, ":")
	if len(addrPort) < 2 {
		addr = req.Host
		port = 80
		if req.TLS != nil {
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

func handleProxyMdnsHosts(
	baseDomain string,
	m mdns.MDNS,
	ifaceName string,
	mdnsDomain string,
	proto mdns.Proto,
	w http.ResponseWriter,
	req *http.Request,
) {
	addr, _, err := getAddrPort(req)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "Error identifying host address and port '%s': %v", req.Host, err)
	}

	host := fmt.Sprintf("%s.%s", strings.TrimSuffix(addr, fmt.Sprintf(".%s", baseDomain)), mdnsDomain)

	ip, err := m.ResolveHost(
		host,
		ifaceName,
		proto,
	)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "Error resolving host '%s': %v", host, err)
	}

	req.URL.Scheme = "http"
	req.URL.User = nil
	req.URL.Host = host
	req.Header["Host"] = []string{host}
	req.Host = host

	httputil.NewSingleHostReverseProxy(&url.URL{
		Scheme: "http",
		Host:   ip.String(),
	}).ServeHTTP(
		w, req,
	)
}

func NewServer(
	addr string,
	baseDomain string,
	ifaceName string,
	service string,
	mdnsDomain string,
	timeout time.Duration,
	disableIPv4 bool,
	disableIPv6 bool,
) (
	http.Server,
	error,
) {
	var listenPortStr string
	hostPort := strings.Split(addr, ":")
	if len(hostPort) < 2 {
		listenPortStr = "80"
	} else {
		listenPortStr = hostPort[len(hostPort)-1]
	}
	listenPort, err := strconv.Atoi(listenPortStr)
	if err != nil {
		return http.Server{}, err
	}

	m := mdns.NewMDNS()

	proto := mdns.ProtoAny
	if disableIPv4 {
		proto = mdns.ProtoInet6
	}
	if disableIPv6 {
		proto = mdns.ProtoInet
	}

	serveMux := http.NewServeMux()
	serveMux.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		hostSlice := strings.Split(req.Host, ":")
		host := hostSlice[0]
		if host == baseDomain {
			handleListMdnsHosts(
				listenPort,
				baseDomain,
				m,
				ifaceName,
				service,
				mdnsDomain,
				timeout,
				proto,
				w,
				req,
			)
			return
		}

		if strings.HasSuffix(host, fmt.Sprintf(".%s", baseDomain)) {
			hostSlice := strings.Split(host, fmt.Sprintf(".%s", baseDomain))
			if len(hostSlice) != 2 {
				http.Error(w, fmt.Sprintf("Bad request: host must be in the format ${mdns_host}.%s, got: %s", baseDomain, host), http.StatusBadRequest)
				return
			}
			mdnsHost := hostSlice[0]
			if len(strings.Split(mdnsHost, ".")) != 1 {
				http.Error(w, fmt.Sprintf("Bad request: host must be in the format ${mdns_host}.%s, got: %s", baseDomain, host), http.StatusBadRequest)
				return
			}
			mdnsHost = fmt.Sprintf("%s.%s", mdnsHost, mdnsDomain)
			req.Header["Host"] = []string{mdnsHost}
			handleProxyMdnsHosts(
				// listenPort,
				baseDomain,
				m,
				ifaceName,
				// service,
				mdnsDomain,
				// timeout,
				proto,
				w,
				req,
			)
			return
		}

		http.Error(w, fmt.Sprintf("Bad request: unexpected host: %s", req.Host), http.StatusBadRequest)
	})

	return http.Server{
		Addr:    addr,
		Handler: serveMux,
	}, nil
}
