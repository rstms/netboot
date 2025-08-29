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

	"embed"
	"github.com/spf13/cobra"
)

//go:embed certs
var Certs embed.FS

//go:embed ipxe
var Ipxe embed.FS

//go:embed mkboot
var Mkboot embed.FS

//go:embed dist
var Dist embed.FS

var rootCmd = &cobra.Command{
	Version: "0.0.13",
	Use:     "netboot",
	Short:   "netboot server daemon",
	Long: `
Run the netboot HTTPS server, serving the netboot API endpoints and the files
requested by the netboot IPXE bootstrap.
`,
	Run: func(cmd *cobra.Command, args []string) {
		options := server.Options{
			Address:   ViperGetString("bind_address"),
			HttpPort:  ViperGetInt("http_port"),
			HttpsPort: ViperGetInt("https_port"),
			Template: &server.Template{
				Certs:  Certs,
				Dist:   Dist,
				Ipxe:   Ipxe,
				Mkboot: Mkboot,
			},
		}
		server, err := server.NewServer("netboot", &options)
		cobra.CheckErr(err)
		var message string
		if ViperGetBool("verbose") {
			message = "CTRL-C to exit"
		}
		err = server.Run(message)
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
	CobraInit(rootCmd)
	OptionString(rootCmd, "bind-address", "a", server.DEFAULT_ADDRESS, "listen bind address")
	OptionString(rootCmd, "http-port", "p", server.DEFAULT_HTTP_PORT, "http listen port")
	OptionString(rootCmd, "https-port", "P", server.DEFAULT_HTTPS_PORT, "https listen port")
	OptionSwitch(rootCmd, "foreground", "f", "run in foreground")
	OptionSwitch(rootCmd, "enable-proxy", "", "enable http proxy")
}
