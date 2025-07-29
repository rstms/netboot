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
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var configCmd = &cobra.Command{
	Use:   "config [edit]",
	Short: "output configuration",
	Long: `
write current configuration file to stdout in YAML format
add comments if --verbose
optional edit command opens current config file in system editor
`,
	Args: cobra.RangeArgs(0, 1),
	Run: func(cmd *cobra.Command, args []string) {

		name := ProgramName()
		if len(args) > 0 {
			switch args[0] {
			case "cat":
			case "file":
				fmt.Println(viper.ConfigFileUsed())
				return
			case "edit":
				editConfigFile()
				return
			case "init":
				initConfigFile()
				return
			default:
				cobra.CheckErr(fmt.Errorf("unknown command: %s", args[0]))
			}
		}
		if ViperGetBool("verbose") {
			currentUser, err := user.Current()
			cobra.CheckErr(err)
			hostname, err := os.Hostname()
			cobra.CheckErr(err)
			fmt.Printf("# %s config\n", name)
			fmt.Printf("# active: %s\n", filepath.Clean(viper.ConfigFileUsed()))
			fmt.Printf("# generated: %s by %s@%s (%s_%s)\n",
				time.Now().Format(time.DateTime),
				currentUser.Username, hostname,
				runtime.GOOS, runtime.GOARCH,
			)

			home, err := os.UserHomeDir()
			cobra.CheckErr(err)
			fmt.Printf("# user_home_dir: %s\n", home)

			userConfig, err := os.UserConfigDir()
			cobra.CheckErr(err)
			fmt.Printf("# default_config_dir: %s\n", filepath.Join(userConfig, name))

			userCache, err := os.UserCacheDir()
			cobra.CheckErr(err)
			fmt.Printf("# default_cache_dir: %s\n", filepath.Join(userCache, name))
			fmt.Println("")
		}

		err := viper.WriteConfigTo(os.Stdout)
		cobra.CheckErr(err)
	},
}

func initConfigFile() {
	file := cfgFile
	if file == "" {
		userConfig, err := os.UserConfigDir()
		cobra.CheckErr(err)
		dir := filepath.Join(userConfig, ProgramName())
		if !IsDir(dir) {
			if !Confirm(fmt.Sprintf("Create directory '%s'", dir)) {
				return
			}
			err := os.Mkdir(dir, 0700)
			cobra.CheckErr(err)
		}
		file = filepath.Join(dir, "config.yaml")
	}
	if IsFile(file) {
		if !Confirm(fmt.Sprintf("Overwrite config file '%s'", file)) {
			return
		}
	}
	ViperSet("force", false)
	err := viper.WriteConfigAs(file)
	cobra.CheckErr(err)
	fmt.Printf("Default configuration written to %s\n", file)
}

func editConfigFile() {
	var editCommand string
	if runtime.GOOS == "windows" {
		editCommand = "notepad"
	} else {
		editCommand = os.Getenv("VISUAL")
		if editCommand == "" {
			editCommand = os.Getenv("EDITOR")
			if editCommand == "" {
				editCommand = "vi"
			}
		}
	}
	editor := exec.Command(editCommand, viper.ConfigFileUsed())
	editor.Stdin = os.Stdin
	editor.Stdout = os.Stdout
	editor.Stderr = os.Stderr
	err := editor.Run()
	cobra.CheckErr(err)
}

func init() {
	rootCmd.AddCommand(configCmd)
}
