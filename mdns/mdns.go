package mdns

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/fornellas/slogxt/log"
	"github.com/godbus/dbus/v5"
	"github.com/holoplot/go-avahi"
)

type Service struct {
	Interface string
	Protocol  Proto
	Name      string
	Type      string
	Domain    string
	Host      string
	IP        net.IP
	Port      uint16
}

func NewServiceFromAvahi(service avahi.Service) (Service, error) {
	iface, err := net.InterfaceByIndex(int(service.Interface))
	if err != nil {
		return Service{}, err
	}

	ip := net.ParseIP(service.Address)
	if ip == nil {
		return Service{}, fmt.Errorf("invalid IP: %v", service.Address)
	}

	return Service{
		Interface: iface.Name,
		Protocol:  Proto(service.Protocol),
		Name:      service.Name,
		Type:      service.Type,
		Domain:    service.Domain,
		Host:      service.Host,
		IP:        ip,
		Port:      service.Port,
	}, nil
}

var AnyIface = "any"

type Proto int32

var ProtoAny = Proto(avahi.ProtoUnspec)
var ProtoInet = Proto(avahi.ProtoInet)
var ProtoInet6 = Proto(avahi.ProtoInet6)

func (p Proto) String() string {
	switch p {
	case ProtoAny:
		return "any"
	case ProtoInet:
		return "inet"
	case ProtoInet6:
		return "inet6"
	default:
		panic(fmt.Sprintf("invalid protocol: %d", p))
	}
}

type MDNS struct {
}

func NewMDNS() (*MDNS, error) {
	var m MDNS
	return &m, nil
}

func (m *MDNS) Close() error {
	return nil
}

func getIfaceIdxFromName(ifaceName string) (int32, error) {
	var iface int32
	iface = avahi.InterfaceUnspec
	if ifaceName != AnyIface {
		var err error
		netIface, err := net.InterfaceByName(ifaceName)
		if err != nil {
			return 0, err
		}
		iface = int32(netIface.Index)
	}
	return iface, nil
}

func (m *MDNS) BrowseServices(
	ctx context.Context,
	ifaceName string,
	proto Proto,
	serviceType string,
	domain string,
	timeout time.Duration,
) ([]Service, error) {
	ctx, logger := log.MustWithGroupAttrs(
		ctx,
		"MDNS.BrowseServices",
		"ifaceName", ifaceName,
		"proto", proto,
		"serviceType", serviceType,
		"domain", domain,
		"timeout", timeout,
	)
	defer logger.Debug("Finished")

	logger.Debug("Getting interface name")
	var iface int32
	iface, err := getIfaceIdxFromName(ifaceName)
	if err != nil {
		return nil, err
	}

	logger.Debug("Getting D-BUS connection")
	dbusConn, err := dbus.ConnectSystemBus()
	if err != nil {
		return nil, err
	}
	defer func() {
		logger.Debug("Closing D-BUS connection")
		dbusConn.Close()
		logger.Debug("Closed D-BUS connection")
	}()

	logger.Debug("Getting Avahi server connection")
	avahiServer, err := avahi.ServerNew(dbusConn)
	if err != nil {
		return nil, err
	}
	defer func() {
		logger.Debug("Closing Avahi server connection")
		avahiServer.Close()
		logger.Debug("Closed Avahi server connection")
	}()

	logger.Debug("Browsing")
	avahiServiceBrowser, err := avahiServer.ServiceBrowserNew(
		iface,
		int32(proto),
		serviceType,
		domain,
		0,
	)
	if err != nil {
		return nil, err
	}

	var services []Service
	var servicesMu sync.Mutex
	var wg sync.WaitGroup
	timeoutCh := time.After(timeout)
	var done bool
	for {
		select {
		case avahiService := <-avahiServiceBrowser.AddChannel:
			wg.Go(func() {
				_, logger := log.MustWithGroup(ctx, avahiService.Name)
				logger.Debug("Resolving Avahi service")
				avahiService, err = avahiServer.ResolveService(
					avahiService.Interface,
					avahiService.Protocol,
					avahiService.Name,
					avahiService.Type,
					avahiService.Domain,
					avahiService.Protocol,
					0,
				)
				logger.Debug("Resolved Avahi service", "service", avahiService)
				if err != nil {
					logger.Warn("Failed to resolve", "err", err)
					return
				}

				service, err := NewServiceFromAvahi(avahiService)
				if err != nil {
					logger.Error("NewServiceFromAvahi", "err", err)
					return
				}

				servicesMu.Lock()
				logger.Debug("Resolved")
				services = append(services, service)
				servicesMu.Unlock()
			})
		case <-timeoutCh:
			logger.Debug("Browse timeout")
			done = true
		}
		if done {
			break
		}
	}

	wg.Wait()

	return services, nil
}

func (m *MDNS) ResolveHost(
	ctx context.Context,
	host string,
	ifaceName string,
	proto Proto,
) (net.IP, error) {
	_, logger := log.MustWithGroupAttrs(
		ctx,
		"MDNS.ResolveHost",
		"host", host,
		"ifaceName", ifaceName,
		"proto", proto,
	)
	defer logger.Debug("Finished")

	var iface int32
	logger.Debug("Getting interface ID")
	iface, err := getIfaceIdxFromName(ifaceName)
	if err != nil {
		return nil, err
	}

	logger.Debug("Getting D-BUS connection")
	dbusConn, err := dbus.ConnectSystemBus()
	if err != nil {
		return nil, err
	}
	defer func() {
		logger.Debug("Closing D-BUS connection")
		dbusConn.Close()
		logger.Debug("Closed D-BUS connection")
	}()

	logger.Debug("Getting Avahi server connection")
	avahiServer, err := avahi.ServerNew(dbusConn)
	if err != nil {
		return nil, err
	}
	defer func() {
		logger.Debug("Closing Avahi server connection")
		avahiServer.Close()
		logger.Debug("Closed Avahi server connection")
	}()

	logger.Debug("Resolving mDNS host")
	hostName, err := avahiServer.ResolveHostName(
		iface,
		int32(proto),
		host,
		int32(proto),
		0,
	)
	if err != nil {
		return nil, err
	}

	ip := net.ParseIP(hostName.Address)
	if ip == nil {
		return nil, fmt.Errorf("invalid IP: %v", hostName.Address)
	}

	logger.Debug("Resolved", "IP", ip)

	return ip, nil
}
