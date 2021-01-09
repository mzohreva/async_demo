package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
)

var serverHost string
var serverPort uint16
var requestTimeout time.Duration
var idleConnectionTimeout time.Duration

var rootCmd = &cobra.Command{
	Use: "client",
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&serverHost, "server", "s", "localhost", "Server host name or IP address")
	rootCmd.PersistentFlags().Uint16VarP(&serverPort, "port", "p", 8080, "Server port")
	rootCmd.PersistentFlags().DurationVar(&requestTimeout, "request-timeout", 60*time.Second, "HTTP request timeout, 0 means no timeout")
	rootCmd.PersistentFlags().DurationVar(&idleConnectionTimeout, "idle-connection-timeout", 0, "Idle connection timeout, 0 means no timeout (default behavior)")
}
