package cli

import (
	"os"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/fornellas/mdns-proxy/cli/lib"
	"github.com/fornellas/mdns-proxy/cli/server"
	"github.com/fornellas/mdns-proxy/cli/version"
	"github.com/fornellas/mdns-proxy/log"
)

var ExitFunc func(int) = func(code int) { os.Exit(code) }

var logLevelStr string
var defaultLogLevelStr = "info"
var forceColor bool
var defaultForceColor = false

var Cmd = &cobra.Command{
	Use:   "mdns-proxy",
	Short: "Go Build Tempmlate.",
	Args:  cobra.NoArgs,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		if forceColor {
			color.NoColor = false
		}
		cmd.SetContext(log.SetLoggerValue(
			cmd.Context(), cmd.OutOrStderr(), logLevelStr, ExitFunc,
		))
	},
	Run: func(cmd *cobra.Command, args []string) {
		logger := log.GetLogger(cmd.Context())
		if err := cmd.Help(); err != nil {
			logger.Fatal(err)
		}
	},
}

var resetFuncs []func()

func Reset() {
	logLevelStr = defaultLogLevelStr
	forceColor = defaultForceColor
	for _, resetFunc := range resetFuncs {
		resetFunc()
	}
	lib.Reset()
}

func init() {
	Cmd.PersistentFlags().StringVarP(
		&logLevelStr, "log-level", "l", defaultLogLevelStr,
		"Logging level",
	)
	Cmd.PersistentFlags().BoolVarP(
		&forceColor, "force-color", "", defaultForceColor,
		"Force colored output",
	)

	Cmd.AddCommand(server.Cmd)
	resetFuncs = append(resetFuncs, server.Reset)

	Cmd.AddCommand(version.Cmd)
	resetFuncs = append(resetFuncs, version.Reset)
}
