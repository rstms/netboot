/*
Copyright © 2025 Matt Krueger <mkrueger@rstms.net>
All rights reserved.

Redistribution and use in source and binary forms, with or without
modification, are permitted provided that the following conditions are met:

 1. Redistributions of source code must retain the above copyright notice,
    this list of conditions and the following disclaimer.

 2. Redistributions in binary form must reproduce the above copyright notice,
    this list of conditions and the following disclaimer in the documentation
    and/or other materials provided with the distribution.

 3. Neither the name of the copyright holder nor the names of its contributors
    may be used to endorse or promote products derived from this software
    without specific prior written permission.

THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS"
AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE
IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE
ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE
LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR
CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF
SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS
INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN
CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE)
ARISING IN ANY WAY OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE
POSSIBILITY OF SUCH DAMAGE.
*/
package cmd

import (
	"github.com/rstms/netboot/server"
	"os"

	"github.com/spf13/cobra"
)

var cfgFile string

const ProgramName = "netboot"

var rootCmd = &cobra.Command{
	Version: "0.0.3",
	Use:     "netboot",
	Short:   "netboot server daemon",
	Long: `
Run the netboot HTTPS server, serving the netboot API endpoints and the files
requested by the netboot IPXE bootstrap.
`,
	Run: func(cmd *cobra.Command, args []string) {
		options := server.Options{
			Hostname: ViperGetString("hostname"),
			Address:  ViperGetString("address"),
			Port:     ViperGetInt("port"),
		}
		server, err := server.NewServer(&options)
		cobra.CheckErr(err)
		err = server.Start()
		cobra.CheckErr(err)
		err = server.Wait()
		cobra.CheckErr(err)
	},
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}
func init() {
	cobra.OnInitialize(InitConfig)
	OptionString(rootCmd, "logfile", "l", "", "log filename")
	OptionString(rootCmd, "config", "c", "", "config file")
	OptionSwitch(rootCmd, "debug", "d", "produce debug output")
	OptionSwitch(rootCmd, "verbose", "v", "increase verbosity")
	OptionString(rootCmd, "hostname", "H", server.DEFAULT_HOSTNAME, "TLS hostname")
	OptionString(rootCmd, "address", "a", server.DEFAULT_ADDRESS, "listen address")
	OptionString(rootCmd, "port", "p", server.DEFAULT_PORT, "listen port")
	OptionSwitch(rootCmd, "foreground", "f", "run in foreground")
}
