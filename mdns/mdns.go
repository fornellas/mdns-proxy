package mdns

import (
	"fmt"
	"net"
	"time"

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
	dbusConn    *dbus.Conn
	avahiServer *avahi.Server
}

func NewMDNS() (*MDNS, error) {
	var m MDNS
	var err error

	m.dbusConn, err = dbus.SystemBus()
	if err != nil {
		return nil, err
	}

	m.avahiServer, err = avahi.ServerNew(m.dbusConn)
	if err != nil {
		return nil, err
	}

	return &m, nil
}

func (m *MDNS) Close() error {
	m.avahiServer.Close()
	return m.dbusConn.Close()
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
	ifaceName string,
	proto Proto,
	serviceType string,
	domain string,
	timeout time.Duration,
) ([]Service, error) {
	var iface int32
	iface, err := getIfaceIdxFromName(ifaceName)
	if err != nil {
		return nil, err
	}

	sb, err := m.avahiServer.ServiceBrowserNew(
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
		select {
		case avahiService = <-sb.AddChannel:
			avahiService, err = m.avahiServer.ResolveService(
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

	hostName, err := m.avahiServer.ResolveHostName(
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
