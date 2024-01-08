package server

import (
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/fornellas/mdns-proxy/mdns"

	"github.com/fornellas/mdns-proxy/log"
	"github.com/fornellas/mdns-proxy/server"
)

var baseDomain string

var defaultAddr = ":7234"
var addr string

var defaultService = "_prometheus-http._tcp"
var service string

var defaultMdnsDomain = "local"
var mdnsDomain string

var defaultTimeout = time.Second
var timeout time.Duration

var defaultIntterfaceStr = mdns.AnyIface
var interfaceStr string

var defaultWantUnicastResponse = false
var wantUnicastResponse bool

var defaultDisableIPv4 = false
var disableIPv4 bool

var defaultDisableIPv6 = false
var disableIPv6 bool

var Cmd = &cobra.Command{
	Use:   "server",
	Short: "Start a server that proxies requests to discovered mDNS hosts.",
	Args:  cobra.ExactArgs(0),
	Run: func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()

		logger := log.GetLogger(ctx)

		srv, err := server.NewServer(
			addr,
			baseDomain,
			interfaceStr,
			service,
			mdnsDomain,
			timeout,
			disableIPv4,
			disableIPv6,
		)
		if err != nil {
			logrus.Fatalf("Error starting server: %v", err)
		}

		go func() {
			sig := make(chan os.Signal, 1)
			signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
			<-sig

			logger.Info("Shutting down...")
			if err := srv.Shutdown(ctx); err != nil {
				logger.Errorf("Shutdown request failed: %v", err)
			}
		}()

		logger.Infof("Starting server on %s", addr)
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			logger.Fatalf("Server error: %v", err)
		}
		logger.Info("Exiting")
	},
}

func init() {
	Cmd.Flags().StringVarP(
		&baseDomain, "base-domain", "", "",
		"Base domain where the proxy will be accessed",
	)
	Cmd.MarkFlagRequired("base-domain")

	Cmd.Flags().StringVarP(
		&addr, "address", "", defaultAddr,
		"TCP address for the server to listen on.",
	)

	Cmd.PersistentFlags().StringVarP(
		&service, "service", "s", defaultService,
		"Service",
	)

	Cmd.PersistentFlags().StringVarP(
		&mdnsDomain, "mdns-domain", "d", defaultMdnsDomain,
		"mDNS Domain",
	)

	Cmd.PersistentFlags().DurationVarP(
		&timeout, "timeout", "t", defaultTimeout,
		"Timeout",
	)

	Cmd.PersistentFlags().StringVarP(
		&interfaceStr, "interface", "i", defaultIntterfaceStr,
		"Multicast interface to use",
	)

	Cmd.PersistentFlags().BoolVarP(
		&wantUnicastResponse, "want-unicast-response", "w", defaultWantUnicastResponse,
		"Unicast response desired, as per 5.4 in RFC",
	)

	Cmd.PersistentFlags().BoolVarP(
		&disableIPv4, "disable-ipv4", "", defaultDisableIPv4,
		"Whether to disable usage of IPv4 for MDNS operations. Does not affect discovered addresses.",
	)

	Cmd.PersistentFlags().BoolVarP(
		&disableIPv6, "disable-ipv6", "", defaultDisableIPv6,
		"Whether to disable usage of IPv6 for MDNS operations. Does not affect discovered addresses.",
	)
}

func Reset() {
	addr = defaultAddr
	service = defaultService
	mdnsDomain = defaultMdnsDomain
	timeout = defaultTimeout
	interfaceStr = defaultIntterfaceStr
	wantUnicastResponse = defaultWantUnicastResponse
	disableIPv4 = defaultDisableIPv4
	disableIPv6 = defaultDisableIPv6
}
