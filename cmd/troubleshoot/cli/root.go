package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/replicatedhq/troubleshoot/cmd/util"
	"github.com/replicatedhq/troubleshoot/internal/traces"
	"github.com/replicatedhq/troubleshoot/pkg/k8sutil"
	"github.com/replicatedhq/troubleshoot/pkg/logger"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"k8s.io/klog/v2"
)

func RootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "support-bundle [urls...]",
		Args:  cobra.MinimumNArgs(0),
		Short: "Generate a support bundle",
		Long: `A support bundle is an archive of files, output, metrics and state
from a server that can be used to assist when troubleshooting a Kubernetes cluster.`,
		SilenceUsage: true,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			v := viper.GetViper()
			v.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
			v.BindPFlags(cmd.Flags())

			logger.SetupLogger(v)

			if err := util.StartProfiling(); err != nil {
				klog.Errorf("Failed to start profiling: %v", err)
			}
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			v := viper.GetViper()

			closer, err := traces.ConfigureTracing("support-bundle")
			if err != nil {
				// Do not fail running support-bundle if tracing fails
				klog.Errorf("Failed to initialize open tracing provider: %v", err)
			} else {
				defer closer()
			}

			err = runTroubleshoot(v, args)
			if v.GetBool("debug") || v.IsSet("v") {
				fmt.Printf("\n%s", traces.GetExporterInstance().GetSummary())
			}

			return err
		},
		PersistentPostRun: func(cmd *cobra.Command, args []string) {
			if err := util.StopProfiling(); err != nil {
				klog.Errorf("Failed to stop profiling: %v", err)
			}
		},
	}

	cobra.OnInitialize(initConfig)

	cmd.AddCommand(Analyze())
	cmd.AddCommand(Redact())
	cmd.AddCommand(VersionCmd())

	cmd.Flags().StringSlice("redactors", []string{}, "names of the additional redactors to use")
	cmd.Flags().Bool("redact", true, "enable/disable default redactions")
	cmd.Flags().Bool("interactive", true, "enable/disable interactive mode")
	cmd.Flags().Bool("collect-without-permissions", true, "always generate a support bundle, even if it some require additional permissions")
	cmd.Flags().StringSliceP("selector", "l", []string{"troubleshoot.sh/kind=support-bundle"}, "selector to filter on for loading additional support bundle specs found in secrets within the cluster")
	cmd.Flags().Bool("load-cluster-specs", false, "enable/disable loading additional troubleshoot specs found within the cluster. required when no specs are provided on the command line")
	cmd.Flags().String("since-time", "", "force pod logs collectors to return logs after a specific date (RFC3339)")
	cmd.Flags().String("since", "", "force pod logs collectors to return logs newer than a relative duration like 5s, 2m, or 3h.")
	cmd.Flags().StringP("output", "o", "", "specify the output file path for the support bundle")
	cmd.Flags().Bool("debug", false, "enable debug logging. This is equivalent to --v=0")

	// hidden in favor of the `insecure-skip-tls-verify` flag
	cmd.Flags().Bool("allow-insecure-connections", false, "when set, do not verify TLS certs when retrieving spec and reporting results")
	cmd.Flags().MarkHidden("allow-insecure-connections")

	// `no-uri` references the `followURI` functionality where we can use an upstream spec when creating a support bundle
	// This flag makes sure we can also disable this and fall back to the default spec.
	cmd.Flags().Bool("no-uri", false, "When this flag is used, Troubleshoot does not attempt to retrieve the bundle referenced by the uri: field in the spec.`")

	k8sutil.AddFlags(cmd.Flags())

	// Initialize klog flags
	logger.InitKlogFlags(cmd)

	// CPU and memory profiling flags
	util.AddProfilingFlags(cmd)

	return cmd
}

func InitAndExecute() {
	if err := RootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}

func initConfig() {
	viper.SetEnvPrefix("TROUBLESHOOT")
	viper.AutomaticEnv()
}
