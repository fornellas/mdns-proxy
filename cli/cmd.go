package cli

import (
	"os"

	"github.com/spf13/cobra"

	slogxtCobra "github.com/fornellas/slogxt/cobra"
	"github.com/fornellas/slogxt/log"
)

var Exit func(int) = func(code int) { os.Exit(code) }

var logLevelStr string
var defaultLogLevelStr = "info"
var forceColor bool
var defaultForceColor = false

var Cmd = &cobra.Command{
	Use:   "mdns-proxy",
	Short: "Go Build Tempmlate.",
	Args:  cobra.NoArgs,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		ctx := log.WithLogger(
			cmd.Context(),
			slogxtCobra.GetLogger(cmd.OutOrStderr()),
			// TODO add With("opt", value),
		)
		cmd.SetContext(ctx)
	},
	Run: func(cmd *cobra.Command, args []string) {
		logger := log.MustLogger(cmd.Context())
		if err := cmd.Help(); err != nil {
			logger.Error("failed to display help", "error", err)
			Exit(1)
		}
	},
}

var resetFlagsFns = []func(){
	func() { slogxtCobra.Reset() },
}

func ResetFlags() {
	for _, resetFlagFn := range resetFlagsFns {
		resetFlagFn()
	}
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
	Cmd.AddCommand(ServerCmd)

	resetFlagsFns = append(resetFlagsFns, func() {
		logLevelStr = defaultLogLevelStr
		forceColor = defaultForceColor
	})
}
