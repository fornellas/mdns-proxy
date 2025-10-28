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

var defaultListenAddr = ":7234"
var listenAddr string

var defaultMdnsService = "_http._tcp"
var mdnsService string

var defaultMdnsDomain = "local"
var mdnsDomain string

var defaultMdnsBrowseTimeout = time.Second
var mdnsBrowseTimeout time.Duration

var defaultMdnsIntterfaceName = mdns.AnyIface
var mdnsInterfaceName string

var defaultMdnsDisableIPv4 = false
var mdnsDisableIPv4 bool

var defaultMdnsDisableIPv6 = false
var mdnsDisableIPv6 bool

var ServerCmd = &cobra.Command{
	Use:   "server",
	Short: "Start a server that proxies requests to discovered mDNS hosts.",
	Args:  cobra.ExactArgs(0),
	Run: func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()

		ctx, logger := log.MustWithGroupAttrs(
			ctx,
			"server",
			"base-domain", baseDomain,
			"listen-addr", listenAddr,
			"mdns-service", mdnsService,
			"mdns-domain", mdnsDomain,
			"mdns-browse-timeout", mdnsBrowseTimeout,
			"mdns-interface-name", mdnsInterfaceName,
			"mdns-disable-ipv4", mdnsDisableIPv4,
			"mdns-disable-ipv6", mdnsDisableIPv6,
		)
		cmd.SetContext(ctx)

		srv, err := server.NewServer(
			ctx,
			server.Config{
				ListenAddr:        listenAddr,
				BaseDomain:        baseDomain,
				MdnsInterfaceName: mdnsInterfaceName,
				MdnsService:       mdnsService,
				MdnsDomain:        mdnsDomain,
				MdnsBrowseTimeout: mdnsBrowseTimeout,
				MdnsDisableIPv4:   mdnsDisableIPv4,
				MdnsDisableIPv6:   mdnsDisableIPv6,
			},
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
	ServerCmd.Flags().StringVarP(
		&baseDomain, "base-domain", "", "",
		"Base domain where the proxy will be accessed",
	)
	ServerCmd.MarkFlagRequired("base-domain")

	ServerCmd.Flags().StringVarP(
		&listenAddr, "listen-address", "", defaultListenAddr,
		"TCP address for the server to listen on.",
	)

	ServerCmd.PersistentFlags().StringVarP(
		&mdnsService, "mdns-service", "s", defaultMdnsService,
		"mDNS Service",
	)

	ServerCmd.PersistentFlags().StringVarP(
		&mdnsDomain, "mdns-domain", "d", defaultMdnsDomain,
		"mDNS Domain",
	)

	ServerCmd.PersistentFlags().DurationVarP(
		&mdnsBrowseTimeout, "mdns-timeout", "t", defaultMdnsBrowseTimeout,
		"mDNS browse timeout",
	)

	ServerCmd.PersistentFlags().StringVarP(
		&mdnsInterfaceName, "mdns-interface-name", "i", defaultMdnsIntterfaceName,
		"mDNS multicast interface to use",
	)

	ServerCmd.PersistentFlags().BoolVarP(
		&mdnsDisableIPv4, "mdns-disable-ipv4", "", defaultMdnsDisableIPv4,
		"Whether to disable usage of IPv4 for mDNS operations. Does not affect discovered addresses.",
	)

	ServerCmd.PersistentFlags().BoolVarP(
		&mdnsDisableIPv6, "mdns-disable-ipv6", "", defaultMdnsDisableIPv6,
		"Whether to disable usage of IPv6 for mDNS operations. Does not affect discovered addresses.",
	)
}

func init() {
	Cmd.AddCommand(ServerCmd)
	resetFlagsFns = append(resetFlagsFns, func() {
		listenAddr = defaultListenAddr
		mdnsService = defaultMdnsService
		mdnsDomain = defaultMdnsDomain
		mdnsBrowseTimeout = defaultMdnsBrowseTimeout
		mdnsInterfaceName = defaultMdnsIntterfaceName
		mdnsDisableIPv4 = defaultMdnsDisableIPv4
		mdnsDisableIPv6 = defaultMdnsDisableIPv6
	})
}
