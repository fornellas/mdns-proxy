package cli

import (
	"os"

	"github.com/spf13/cobra"

	slogxtCobra "github.com/fornellas/slogxt/cobra"
	"github.com/fornellas/slogxt/log"
)

var Exit func(int) = func(code int) { os.Exit(code) }

var Cmd = &cobra.Command{
	Use:   "mdns-proxy",
	Short: "Go Build Tempmlate.",
	Args:  cobra.NoArgs,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		logger := slogxtCobra.GetLogger(cmd.OutOrStderr())
		ctx := log.WithLogger(cmd.Context(), logger)
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

var resetFlagsFns = []func(){}

func ResetFlags() {
	for _, resetFlagFn := range resetFlagsFns {
		resetFlagFn()
	}
}

func init() {
	slogxtCobra.AddLoggerFlags(Cmd)

	resetFlagsFns = append(resetFlagsFns, func() {
		slogxtCobra.Reset()
	})
}
