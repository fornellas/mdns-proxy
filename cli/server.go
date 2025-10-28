package cli

import (
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/fornellas/mdns-proxy/mdns"

	"github.com/fornellas/slogxt/log"

	"github.com/fornellas/mdns-proxy/server"
)

var baseDomain string

var defaultAddr = ":7234"
var addr string

var defaultService = "_http._tcp"
var service string

var defaultMdnsDomain = "local"
var mdnsDomain string

var defaultTimeout = time.Second
var timeout time.Duration

var defaultIntterfaceStr = mdns.AnyIface
var interfaceStr string

var defaultDisableIPv4 = false
var disableIPv4 bool

var defaultDisableIPv6 = false
var disableIPv6 bool

var ServerCmd = &cobra.Command{
	Use:   "server",
	Short: "Start a server that proxies requests to discovered mDNS hosts.",
	Args:  cobra.ExactArgs(0),
	Run: func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()

		ctx, logger := log.MustWithGroupAttrs(
			ctx,
			"Server",
			"base-domain", baseDomain,
			"addr", addr,
			"service", service,
			"mdns-domain", mdnsDomain,
			"timeout", timeout,
			"interface", interfaceStr,
			"disable-ipv4", disableIPv4,
			"disable-ipv6", disableIPv6,
		)
		cmd.SetContext(ctx)

		srv, err := server.NewServer(
			ctx,
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
			logger.Error("Error starting server", "err", err)
			Exit(1)
		}

		go func() {
			sig := make(chan os.Signal, 1)
			signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
			<-sig

			logger.Info("Shutting down...")
			if err := srv.Shutdown(ctx); err != nil {
				logger.Error("Shutdown request failed", "err", err)
			}
		}()

		logger.Info("Starting server")
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			logger.Error("Listen and server error", "err", err)
			Exit(1)
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
		&disableIPv4, "disable-ipv4", "", defaultDisableIPv4,
		"Whether to disable usage of IPv4 for MDNS operations. Does not affect discovered addresses.",
	)

	Cmd.PersistentFlags().BoolVarP(
		&disableIPv6, "disable-ipv6", "", defaultDisableIPv6,
		"Whether to disable usage of IPv6 for MDNS operations. Does not affect discovered addresses.",
	)
}

func init() {
	resetFlagsFns = append(resetFlagsFns, func() {
		addr = defaultAddr
		service = defaultService
		mdnsDomain = defaultMdnsDomain
		timeout = defaultTimeout
		interfaceStr = defaultIntterfaceStr
		disableIPv4 = defaultDisableIPv4
		disableIPv6 = defaultDisableIPv6
	})
}
