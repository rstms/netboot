package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"log"
	"os"
	"path/filepath"
	"strings"
)

var ViperPrefix = ProgramName() + ".cli."
var LogFile *os.File

var CONFIRM_ACCEPT_MESSAGE = "Proceeding"
var CONFIRM_REJECT_MESSAGE = "Cowardly refused"

func viperKey(name string) string {
	return ViperPrefix + strings.ToLower(strings.ReplaceAll(name, "-", "_"))
}

func ViperGetBool(key string) bool {
	return viper.GetBool(viperKey(key))
}

func ViperGetString(key string) string {
	return ExpandPath(viper.GetString(viperKey(key)))
}

func ViperGetInt(key string) int {
	return viper.GetInt(viperKey(key))
}

func ViperGetInt64(key string) int64 {
	return viper.GetInt64(viperKey(key))
}

func ViperSet(key string, value any) {
	viper.Set(viperKey(key), value)
}

func ViperSetDefault(key string, value any) {
	viper.SetDefault(viperKey(key), value)
}

func OptionSwitch(cmd *cobra.Command, name, flag, description string) {
	if cmd == rootCmd {
		if flag == "" {
			rootCmd.PersistentFlags().Bool(name, false, description)
		} else {
			rootCmd.PersistentFlags().BoolP(name, flag, false, description)
		}
		viper.BindPFlag(viperKey(name), rootCmd.PersistentFlags().Lookup(name))
	} else {
		if flag == "" {
			cmd.Flags().Bool(name, false, description)
		} else {
			cmd.Flags().BoolP(name, flag, false, description)
		}
		prefix := strings.ToLower(strings.ReplaceAll(cmd.Name(), "-", "_")) + "."
		viper.BindPFlag(viperKey(prefix+name), cmd.Flags().Lookup(name))
	}
}

func OptionString(cmd *cobra.Command, name, flag, defaultValue, description string) {

	if cmd == rootCmd {
		if flag == "" {
			rootCmd.PersistentFlags().String(name, defaultValue, description)
		} else {
			rootCmd.PersistentFlags().StringP(name, flag, defaultValue, description)
		}

		viper.BindPFlag(viperKey(name), rootCmd.PersistentFlags().Lookup(name))
	} else {
		if flag == "" {
			cmd.PersistentFlags().String(name, defaultValue, description)
		} else {
			cmd.PersistentFlags().StringP(name, flag, defaultValue, description)
		}
		prefix := strings.ToLower(strings.ReplaceAll(cmd.Name(), "-", "_")) + "."
		viper.BindPFlag(viperKey(prefix+name), cmd.PersistentFlags().Lookup(name))
	}
}

func OpenLog() {
	filename := ViperGetString("logfile")
	LogFile = nil
	if filename == "stdout" || filename == "-" {
		log.SetOutput(os.Stdout)
	} else if filename == "stderr" || filename == "" {
		log.SetOutput(os.Stderr)
	} else {
		fp, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0660)
		if err != nil {
			log.Fatalf("failed opening log file: %v", err)
		}
		LogFile = fp
		log.SetOutput(LogFile)
		log.SetPrefix(fmt.Sprintf("[%d] ", os.Getpid()))
		log.SetFlags(log.Ldate | log.Ltime | log.Lmsgprefix)
		log.Printf("%s v%s startup\n", rootCmd.Name(), rootCmd.Version)
		cobra.OnFinalize(CloseLog)
	}
	if ViperGetBool("debug") {
		log.SetFlags(log.Flags() | log.Lshortfile)
	}
}

func CloseLog() {
	if LogFile != nil {
		log.Println("shutdown")
		err := LogFile.Close()
		cobra.CheckErr(err)
		LogFile = nil
	}
}

func FormatJSON(v any) string {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		log.Fatalf("failed formatting JSON: %v", err)
	}
	return string(data)
}

func IsDir(path string) bool {
	stat, err := os.Stat(path)
	if err != nil {
		return false
	}
	return stat.IsDir()
}

func IsFile(pathname string) bool {
	_, err := os.Stat(pathname)
	return !os.IsNotExist(err)
}

func ExpandPath(pathname string) string {
	if len(pathname) > 1 && pathname[0] == '~' {
		home, err := os.UserHomeDir()
		if err != nil {
			log.Fatalf("failed getting user home dir: %v", err)
		}
		pathname = filepath.Join(home, pathname[1:])
	}
	pathname = os.ExpandEnv(pathname)
	return pathname
}

func ProgramName() string {
	return strings.ToLower(strings.ReplaceAll(rootCmd.Name(), "-", "_"))
}

func InitConfig() {
	viper.SetEnvPrefix(ProgramName())
	viper.AutomaticEnv()
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)
		userConfig, err := os.UserConfigDir()
		cobra.CheckErr(err)
		viper.AddConfigPath(filepath.Join(home, "."+ProgramName()))
		viper.AddConfigPath(filepath.Join(userConfig, ProgramName()))
		viper.AddConfigPath(filepath.Join("/etc", ProgramName()))
		viper.SetConfigType("yaml")
		viper.SetConfigName("config")
	}
	err := viper.ReadInConfig()
	if err != nil {
		_, ok := err.(viper.ConfigFileNotFoundError)
		if !ok {
			cobra.CheckErr(err)
		}
	}
	OpenLog()
	if viper.ConfigFileUsed() != "" && ViperGetBool("verbose") {
		log.Println("Using config file:", viper.ConfigFileUsed())
	}
}

func Confirm(prompt string) bool {
	if ViperGetBool("force") {
		return true
	}
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Printf("%s [y/N]: ", prompt)
		response, err := reader.ReadString('\n')
		cobra.CheckErr(err)
		response = strings.ToLower(strings.TrimSpace(response))
		if response == "y" || response == "yes" {
			msg := ViperGetString("messages.confirm_accept")
			if msg == "" {
				msg = CONFIRM_ACCEPT_MESSAGE
			}
			if msg != "" {
				fmt.Println(msg)
			}
			return true
		} else if response == "n" || response == "no" || response == "" {
			msg := ViperGetString("messages.confirm_reject")
			if msg == "" {
				msg = CONFIRM_REJECT_MESSAGE
			}
			if msg != "" {
				fmt.Println(msg)
			}
			return false
		}
	}
}
