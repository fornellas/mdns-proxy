package mdns

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/holoplot/go-avahi"
	"github.com/sirupsen/logrus"

	"github.com/fornellas/mdns-proxy/log"
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

func newServiceFromAvahi(service avahi.Service) (Service, error) {
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
	logger := log.GetLogger(ctx)
	logger.WithFields(logrus.Fields{
		"ifaceName":   ifaceName,
		"proto":       proto,
		"serviceType": serviceType,
		"domain":      domain,
		"timeout":     timeout,
	}).Info("MDNS.BrowseServices")

	var iface int32
	logger.WithFields(logrus.Fields{
		"ifaceName": ifaceName,
	}).Info("MDNS.getIfaceIdxFromName")
	iface, err := getIfaceIdxFromName(ifaceName)
	if err != nil {
		return nil, err
	}

	logger.Info("SystemBus")
	dbusConn, err := dbus.SystemBus()
	if err != nil {
		return nil, err
	}
	defer func() { dbusConn.Close() }()

	logger.Info("avahi.ServerNew")
	avahiServer, err := avahi.ServerNew(dbusConn)
	if err != nil {
		return nil, err
	}
	defer func() { avahiServer.Close() }()

	logger.Info("avahiServer.ServiceBrowserNew")
	sb, err := avahiServer.ServiceBrowserNew(
		iface,
		int32(proto),
		serviceType,
		domain,
		0,
	)
	if err != nil {
		return nil, err
	}

	var avahiService avahi.Service
	var services []Service
	timeoutCh := time.After(timeout)
	var done bool
	for {
		logger.Info("for")
		select {
		case avahiService = <-sb.AddChannel:
			logger.Info("<-sb.AddChannel")
			logger.Info("avahiServer.ResolveService")
			avahiService, err = avahiServer.ResolveService(
				avahiService.Interface,
				avahiService.Protocol,
				avahiService.Name,
				avahiService.Type,
				avahiService.Domain,
				avahiService.Protocol,
				0,
			)
			if err != nil {
				return nil, err
			}

			service, err := newServiceFromAvahi(avahiService)
			if err != nil {
				return nil, err
			}

			services = append(services, service)
		case <-timeoutCh:
			logger.Info("<-timeoutCh")
			done = true
		}
		if done {
			break
		}
	}

	return services, nil
}

func (m *MDNS) ResolveHost(
	host string,
	ifaceName string,
	proto Proto,
) (net.IP, error) {
	var iface int32
	iface, err := getIfaceIdxFromName(ifaceName)
	if err != nil {
		return nil, err
	}

	dbusConn, err := dbus.SystemBus()
	if err != nil {
		return nil, err
	}
	defer func() { dbusConn.Close() }()

	avahiServer, err := avahi.ServerNew(dbusConn)
	if err != nil {
		return nil, err
	}
	defer func() { avahiServer.Close() }()

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

	return ip, nil
}
