package main

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"io/ioutil"
	"os"
	"strconv"
	"time"

	"github.com/virtualzone/onedrive-uploader/sdk"
)

type CommandFunction func(client *sdk.Client, renderer *OutputRenderer, args []string)

type CommandFunctionDefinition struct {
	Fn              CommandFunction
	MinArgs         int
	InitSecretStore bool
	RequireConfig   bool
}

var (
	commands = map[string]*CommandFunctionDefinition{
		"config":   {Fn: cmdConfig, MinArgs: 0, InitSecretStore: false, RequireConfig: false},
		"login":    {Fn: cmdLogin, MinArgs: 0, InitSecretStore: false, RequireConfig: true},
		"mkdir":    {Fn: cmdCreateDir, MinArgs: 1, InitSecretStore: true, RequireConfig: true},
		"up":   {Fn: cmdUpload, MinArgs: 1, InitSecretStore: true, RequireConfig: true},
		"down": {Fn: cmdDownload, MinArgs: 1, InitSecretStore: true, RequireConfig: true},
		"rm":       {Fn: cmdDelete, MinArgs: 1, InitSecretStore: true, RequireConfig: true},
		"ls":       {Fn: cmdList, MinArgs: 0, InitSecretStore: true, RequireConfig: true},
		"info":     {Fn: cmdInfo, MinArgs: 1, InitSecretStore: true, RequireConfig: true},
		"sha1":     {Fn: cmdSHA1, MinArgs: 1, InitSecretStore: true, RequireConfig: true},
		"search":   {Fn: cmdSearch, MinArgs: 1, InitSecretStore: true, RequireConfig: true},
		"sha256":   {Fn: cmdSHA256, MinArgs: 1, InitSecretStore: true, RequireConfig: true},
		"migrate":  {Fn: cmdMigrateConfig, MinArgs: 1, InitSecretStore: false, RequireConfig: false},
		"version":  {Fn: cmdVersion, MinArgs: 0, InitSecretStore: false, RequireConfig: false},
		"share":    {Fn: cmdShare, MinArgs: 1, InitSecretStore: true, RequireConfig: true},
	}
)
type Link struct {
    WebUrl string `json:"webUrl"`
}

type Response struct {
    Link Link `json:"link"`
}
func cmdShare(client *sdk.Client, renderer *OutputRenderer, args []string) {
    body := client.Share(args[0])
    var respon Response
    err := json.Unmarshal([]byte(body), &respon)
    if err != nil {
        fmt.Println("Error unmarshaling JSON:", err)
        return
    }
    fmt.Println(respon.Link.WebUrl)
    // print(string(body.link.webUrl))
}
func cmdSearch(client *sdk.Client, renderer *OutputRenderer, args []string) {
	if len(args) >1 {
	    print("Too much !!")
	}
	renderer.initSpinner("Searching for file...")
	list, err := client.Search(args[0])
	renderer.stopSpinner()
	if err != nil {
		logError("Could not list: " + err.Error())
		return
	}
	for _, item := range list {
	    itemType := "d"
	    if item.File.MimeType != "" {
		    itemType = "f"
	    }
	    fmt.Println(itemType, item.Name," <", formatSize(float64(item.SizeBytes)),"> ")
	}
}
func cmdConfig(client *sdk.Client, renderer *OutputRenderer, args []string) {
	targetPath, err := findConfigFilePath()
	if err != nil {
		logError("Could not init config path: " + err.Error())
		return
	}
	interactiveConfig := &InteractiveConfig{
		TargetPath: targetPath,
	}
	interactiveConfig.Run()
}

func cmdMigrateConfig(client *sdk.Client, renderer *OutputRenderer, args []string) {
	type secretStore struct {
		AccessToken  string    `json:"access_token"`
		RefreshToken string    `json:"refresh_token"`
		Expiry       time.Time `json:"expiry"`
	}

	readSecretJson := func(filename string) (*secretStore, error) {
		data, err := ioutil.ReadFile(filename)
		if err != nil {
			return nil, err
		}
		var config secretStore
		if err := json.Unmarshal(data, &config); err != nil {
			return nil, err
		}
		return &config, nil
	}

	sourceConfig, err := sdk.ReadConfig(args[0])
	if err != nil {
		logError("Could not read source config: " + err.Error())
		return
	}
	sourceSecret, err := readSecretJson(sourceConfig.SecretStore)
	if err != nil {
		logError("Could not read source secret: " + err.Error())
		return
	}
	targetPath, err := findConfigFilePath()
	if err != nil {
		logError("Could not init target config: " + err.Error())
		return
	}
	targetConfig := &sdk.Config{
		ConfigFilePath: targetPath,
		ClientID:       sourceConfig.ClientID,
		ClientSecret:   sourceConfig.ClientSecret,
		Scopes:         sourceConfig.Scopes,
		RedirectURL:    sourceConfig.RedirectURL,
		Root:           sourceConfig.Root,
		AccessToken:    sourceSecret.AccessToken,
		RefreshToken:   sourceSecret.RefreshToken,
		Expiry:         sourceSecret.Expiry,
	}
	if err := targetConfig.Write(); err != nil {
		logError("Could not write target config: " + err.Error())
		return
	}
	log("Configuration migrated.")
}

func cmdLogin(client *sdk.Client, renderer *OutputRenderer, args []string) {
	log("------------------------------------")
	log("Open a browser and go to:")
	print(client.GetLoginURL())
	log("------------------------------------")
	renderer.initSpinner("Waiting for code...")
	err := client.Login()
	renderer.stopSpinner()
	if err != nil {
		logError("Could not log in: " + err.Error())
		return
	}
	log("Login successful.")
}

func cmdCreateDir(client *sdk.Client, renderer *OutputRenderer, args []string) {
	renderer.initSpinner("Creating directory...")
	err := client.CreateDir(args[0])
	renderer.stopSpinner()
	if err != nil {
		logError("Could not create folder: " + err.Error())
		return
	}
	log("Folder created.")
}

func cmdUpload(client *sdk.Client, renderer *OutputRenderer, args []string) {
	path := args[len(args)-1]
	sourceFiles := args[:len(args)-1]
	fmt.Println(sourceFiles)
	numFiles := 0
	for _, sourceFile := range sourceFiles {
		fileStat, err := os.Stat(sourceFile)
		if err != nil {
			logError("Could not get stats for local file: " + err.Error())
			return
		}
		// Skip directories
		if fileStat.IsDir() {
			continue
		}
		// Upload file
		numFiles++
		client.ResetChannels()
		done := false
		go func() {
			fileStat := <-client.ChannelTransferStart
			renderer.initProgressBar(fileStat.Size(), "Uploading "+fmt.Sprintf("%-20s", cutString(fileStat.Name(), 17)+"..."))
		}()
		go func() {
			for !done {
				bytes := <-client.ChannelTransferProgress
				renderer.updateProgressBar(bytes)
			}
		}()
		go func() {
			done = <-client.ChannelTransferFinish
		}()
		err = client.Upload(sourceFile, path)
		if err != nil {
			logError("Could not upload file: " + err.Error())
			return
		}
	}
	if numFiles == 0 {
		logError("No files for uploading specified (uploading directories is not supported)")
	}
}

func cmdDownload(client *sdk.Client, renderer *OutputRenderer, args []string) {
	done := false
	var path2 string
	go func() {
		var fileStat fs.FileInfo = nil
		for fileStat == nil {
			fileStat := <-client.ChannelTransferStart
			if fileStat == nil {
				renderer.initSpinner("Retrieving information...")
			} else {
				renderer.stopSpinner()
				renderer.initProgressBar(fileStat.Size(), "Downloading "+fileStat.Name()+"...")
			}
		}
	}()
	go func() {
		for !done {
			bytes := <-client.ChannelTransferProgress
			renderer.updateProgressBar(bytes)
		}
	}()
	go func() {
		done = <-client.ChannelTransferFinish
	}()
	if len(args) > 1 {
		path2 = args[1]
	}else{
	    path2 = "."
	}
	err := client.Download(args[0], path2)
	if err != nil {
		logError("Could not download file: " + err.Error())
		return
	}
	log("File downloaded.")
}

func cmdDelete(client *sdk.Client, renderer *OutputRenderer, args []string) {
	renderer.initSpinner("Deleting...")
	err := client.Delete(args[0])
	renderer.stopSpinner()
	if err != nil {
		logError("Could not delete: " + err.Error())
		return
	}
	log("Deleted.")
}
func formatSize(bytes float64) string {
    const (
        KB = 1024
        MB = KB * 1024
        GB = MB * 1024
        TB = GB * 1024
    )

    if bytes < KB {
        return fmt.Sprintf("%.0f bytes", bytes)
    } else if bytes < MB {
        return fmt.Sprintf("%.2f KB", bytes/KB)
    } else if bytes < GB {
        return fmt.Sprintf("%.2f MB", bytes/MB)
    } else if bytes < TB {
        return fmt.Sprintf("%.2f GB", bytes/GB)
    } else {
        return fmt.Sprintf("%.2f TB", bytes/TB)
    }
}
func cmdList(client *sdk.Client, renderer *OutputRenderer, args []string) {
	var path string
	if len(args) >0 {
	    path = args[0]
	}else{
	    path = "/"
	}
	renderer.initSpinner("Retrieving directory listing...")
	list, err := client.List(path)
	renderer.stopSpinner()
	if err != nil {
		logError("Could not list: " + err.Error())
		return
	}
	for _, item := range list {
	    itemType := "d"
	    if item.File.MimeType != "" {
		    itemType = "f"
	    }
	    fmt.Println(itemType, item.Name," <", formatSize(float64(item.SizeBytes)),"> ")
	}

}

func cmdInfo(client *sdk.Client, renderer *OutputRenderer, args []string) {
	renderer.initSpinner("Retrieving information...")
	item, err := client.Info(args[0])
	renderer.stopSpinner()
	if err != nil {
		logError("Could not get info: " + err.Error())
		return
	}
	itemType := "folder"
	if item.Type == sdk.DriveItemTypeFile {
		itemType = "file"
	}
	print("Type:           " + itemType)
	print("Size:           " + strconv.FormatInt(item.SizeBytes, 10) + " bytes")
	if itemType == "folder" {
		print("Child Count:    " + strconv.Itoa(item.Folder.ChildCount))
	} else {
		print("MIME Type:      " + item.File.MimeType)
		print("SHA1 Hash:      " + item.File.Hashes.SHA1)
		print("SHA256 Hash:    " + item.File.Hashes.SHA256)
		print("Quick XOR Hash: " + item.File.Hashes.QuickXOR)
	}
}

func cmdSHA1(client *sdk.Client, renderer *OutputRenderer, args []string) {
	renderer.initSpinner("Retrieving SHA1 hash...")
	item, err := client.Info(args[0])
	renderer.stopSpinner()
	if err != nil {
		logError("Could not get info: " + err.Error())
		return
	}
	print(item.File.Hashes.SHA1)
}

func cmdSHA256(client *sdk.Client, renderer *OutputRenderer, args []string) {
	renderer.initSpinner("Retrieving SHA256 hash...")
	item, err := client.Info(args[0])
	renderer.stopSpinner()
	if err != nil {
		logError("Could not get info: " + err.Error())
		return
	}
	print(item.File.Hashes.SHA256)
}

func cmdVersion(client *sdk.Client, renderer *OutputRenderer, args []string) {
	print(AppVersion)
}

func cutString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}
