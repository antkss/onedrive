package sdk

import (
	"errors"
	"net/http"
	"strings"
	"fmt"
	"encoding/json"
	"log"
)
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
type User struct {
	DisplayName string `json:"displayName"`
	Email       string `json:"email,omitempty"`
	ID          string `json:"id,omitempty"`
}

type Quota struct {
	Deleted   int64  `json:"deleted"`
	Remaining int64  `json:"remaining"`
	State     string `json:"state"`
	Total     int64  `json:"total"`
	Used      int64  `json:"used"`
}

type Drive struct {
	OdataContext        string `json:"@odata.context"`
	CreatedDateTime     string `json:"createdDateTime"`
	Description         string `json:"description"`
	ID                  string `json:"id"`
	LastModifiedDateTime string `json:"lastModifiedDateTime"`
	Name                string `json:"name"`
	WebURL              string `json:"webUrl"`
	DriveType           string `json:"driveType"`
	CreatedBy           struct {
		User User `json:"user"`
	} `json:"createdBy"`
	LastModifiedBy struct {
		User User `json:"user"`
	} `json:"lastModifiedBy"`
	Owner struct {
		User User `json:"user"`
	} `json:"owner"`
	Quota Quota `json:"quota"`
}
func (client *Client) Info(path string) (*DriveItem, error) {
	if len(path) > 0 && path[0] == '.' {
		return nil, errors.New("invalid path (should start with /)")
	}
	path = strings.TrimSuffix(path, "/")
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	url := GraphURL + "me" + client.Config.Root + ":" + path
	if path == "/" {
	    url = GraphURL + "me" + "/drive"
	}
	status, data, err := client.httpGet(url, nil)
	var drive Drive
	if path == "/" {
	    err := json.Unmarshal([]byte(data), &drive)
	    if err != nil {
		    log.Fatal(err)
	    }
	    fmt.Println("\n")
	    fmt.Printf("Drive ID: %s\n", drive.ID)
	    fmt.Printf("Drive Name: %s\n", drive.Name)
	    fmt.Printf("Created By: %s\n", drive.CreatedBy.User.DisplayName)
	    fmt.Printf("Last Modified By: %s (%s)\n", drive.LastModifiedBy.User.DisplayName, drive.LastModifiedBy.User.Email)
	    fmt.Printf("Owner: %s (%s)\n", drive.Owner.User.DisplayName, drive.Owner.User.Email)
	    print("Quota Used: " +formatSize(float64(drive.Quota.Used)))
	    print("\nQuota Remaining: "+formatSize(float64(drive.Quota.Remaining))+"\n")
	    return nil,nil
	}
	if err != nil {
		return nil, err
	}
	if status == http.StatusNotFound {
		return nil, errors.New("path not found")
	}
	if status != http.StatusOK {
		return nil, client.handleResponseError(status, data)
	}
	var driveItem DriveItem
	if err := UnmarshalJSON(&driveItem, data); err != nil {
		return nil, err
	}
	if driveItem.File.MimeType != "" {
		driveItem.Type = DriveItemTypeFile
	} else {
		driveItem.Type = DriveItemTypeFolder
	}
	return &driveItem, nil
}
