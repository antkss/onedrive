package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"github.com/virtualzone/onedrive-uploader/sdk"
)

type Flags struct {
	ConfigPath             string
	Verbose                bool
	Quiet                  bool
	UploadSessionRangeSize int
}

var (
	AppFlags = Flags{}
)



func prepareFlags() {
	flag.StringVar(&AppFlags.ConfigPath, "c", "", "path to config.json")
	flag.IntVar(&AppFlags.UploadSessionRangeSize, "u", 320*30, "upload range size in KB (multiple of 320 KB)")
	flag.BoolVar(&AppFlags.Quiet, "q", false, "output errors only")
	flag.BoolVar(&AppFlags.Verbose, "v", false, "verbose output")
	flag.Parse()
}

func logVerbose(s string) {
	if AppFlags.Verbose {
		fmt.Println(s)
	}
}

func print(s string) {
	fmt.Println(s)
}

func log(s string) {
	if !AppFlags.Quiet {
		fmt.Println(s)
	}
}

func logError(s string) {
	fmt.Println(s)
	os.Exit(1)
}

func findConfigFilePath() (string, error) {
	if AppFlags.ConfigPath != "" {
		return AppFlags.ConfigPath, nil
	}
	configPath := GetConfigDir()
	if configPath == "" {
		var err error
		configPath, err = os.Getwd()
		if (err != nil) || (configPath == "") {
			return "", errors.New("could neither get system config dir nor current working dir")
		}
	} else {
		configPath = filepath.Join(configPath, "onedrive")
		_, err := os.Stat(configPath)
		if err != nil && os.IsNotExist(err) {
			os.MkdirAll(configPath, os.FileMode(0700))
		}
	}
	configPath = filepath.Join(configPath, "config.json")
	return configPath, nil
}

func main() {
	prepareFlags()
	cmd := ""
	if flag.NArg() > 0 {
		cmd = strings.ToLower(flag.Args()[0])
	}
	cmdDef := commands[cmd]
	if cmdDef == nil {
	    cmdDef = commands["ls"]
		// print("OneDrive Uploader " + AppVersion)
		// printHelp()
		// return
	}
	logVerbose("OneDrive Uploader " + AppVersion)
	args := []string{}
	if flag.NArg() > 1 {
		args = flag.Args()[1:]
	}
	if len(args) < cmdDef.MinArgs {
		cmdDef = commands["help"]
		return
	}
	outputRenderer := &OutputRenderer{
		Quiet: AppFlags.Quiet,
	}
	var client *sdk.Client = nil
	if cmdDef.RequireConfig {
		configPath, err := findConfigFilePath()
		if (err != nil) || (configPath == "") {
			logError("Could not initialize config path: " + err.Error())
			return
		}
		conf, err := sdk.ReadConfig(configPath)
		if err != nil {
			logError("Could not read config: " + err.Error())
			return
		}
		client = sdk.CreateClient(conf)
		client.UseTransferSignals = true
		client.Verbose = AppFlags.Verbose
		client.UploadSessionRangeSize = AppFlags.UploadSessionRangeSize
		if cmdDef.InitSecretStore {
			logVerbose("Reading secret store...")
			if client.ShouldRenewAccessToken() {
				logVerbose("Renewing access token...")
				outputRenderer.initSpinner("Renewing access token...")
				if _, err := client.RenewAccessToken(); err != nil {
					outputRenderer.stopSpinner()
					logError("Could not renew access token: " + err.Error())
					return
				}
				outputRenderer.stopSpinner()
			}
		}
	}
	cmdDef.Fn(client, outputRenderer, args)
}
