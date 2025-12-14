package main

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"github.com/zscaler/zscaler-sdk-go/v3/zscaler/zcon"
	"github.com/zscaler/zscaler-sdk-go/v3/zscaler/zcon/services"
	"github.com/zscaler/zscaler-sdk-go/v3/zscaler/zcon/services/common"
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	ecGroupEndpoint = "/ecgroup"
	//ecGroupLiteEndpoint = "/ecgroup/lite"
	ENV_DRY_RUN_FLAG = "DRY_RUN"
	JSON_PATH_FOLDER = "zscc-json-output"
)

var (
	DRY_RUN_MODE     = true
	err              error
	JSON_PATH_PREFIX string
	JSON_OUTPUT_PATH *string
	JSON_RAW_BYTES   *[]byte
)

//go:embed assets/*
var assets embed.FS

func getPlatformIcon(platform string) string {
	switch strings.ToUpper(platform) {
	case "AWS":
		return "\uf270" // AWS logo (Nerd Font - Material Design Icons)
	case "GCP":
		return "\ue7b2" // GCP logo (Nerd Font)
	default:
		return "\U000f015f" // Generic cloud icon (Nerd Font)
	}
}

func main() {
	// Check if flag is set in env and if it exists, set this to true, else false
	if val, ok := os.LookupEnv(ENV_DRY_RUN_FLAG); ok {
		DRY_RUN_MODE, err = strconv.ParseBool(val)
		if err != nil {
			log.Panicf("Value passed to %s was unable to be parsed as bool: %s", ENV_DRY_RUN_FLAG, val)
		}
	}
	fmt.Println("Starting Zscaler Cloud Node Cleanup")
	fmt.Println("DRY_RUN_MODE:", DRY_RUN_MODE)

	// Fetch cwd of the app
	cwd, err := os.Getwd()
	if err != nil {
		log.Fatalf("Error getting current working directory: %v", err)
	}

	JSON_PATH_PREFIX = fmt.Sprintf("%s/%s", cwd, JSON_PATH_FOLDER)

	// Confirm that JSON_PATH_PREFIX dir exists, or create if not
	if _, err := os.Stat(JSON_PATH_PREFIX); os.IsNotExist(err) {
		err = os.Mkdir(JSON_PATH_PREFIX, 0755)
		if err != nil {
			log.Fatalf("Error creating JSON_PATH_PREFIX directory: %v", err)
		}
	}

	setupClient()
}

func setupClient() {
	// Read the JSON file from embedded assets
	byteValue, err := assets.ReadFile("assets/.zcon_access")
	if err != nil {
		fmt.Println("Error opening file:", err)
	}

	// Parse the JSON file
	var zconAccess map[string]string
	json.Unmarshal(byteValue, &zconAccess)

	for k, v := range zconAccess {
		os.Setenv(k, v)
	}

	username := os.Getenv("ZCON_USERNAME")
	password := os.Getenv("ZCON_PASSWORD")
	apiKey := os.Getenv("ZCON_API_KEY")
	zconCloud := os.Getenv("ZCON_CLOUD")

	zconCfg, err := zcon.NewConfiguration(
		zcon.WithZconUsername(username),
		zcon.WithZconPassword(password),
		zcon.WithZconAPIKey(apiKey),
		zcon.WithZconCloud(zconCloud),
		//zcon.WithDebug(true),
	)
	if err != nil {
		log.Fatalf("Error creating ZCON configuration: %v", err)
	}

	zconClient, err := zcon.NewClient(zconCfg)
	if err != nil {
		log.Fatalf("Failed to create ZCON client: %v", err)
	}

	service := services.New(zconClient)

	ctx := context.Background()

	var ccgroups []EcGroup
	ccgroups, JSON_RAW_BYTES, err = EcGroupGetAllExtended(ctx, service)
	if err != nil {
		log.Fatalf("Error listing EC Groups: %v", err)
	}

	fmt.Printf("EC Groups: %d\n", len(ccgroups))

	// Extract out the top level list of all the Zscaler Cloud Connector Groups
	for _, group := range ccgroups {
		fmt.Println("########################################")
		fmt.Printf(
			"\n### ⊳ %s %s - Group (%d) ==> %s [%s] with %d ECVMs ###\n\n",
			getPlatformIcon(group.Platform),
			group.Platform,
			group.ID,
			group.Name,
			group.AwsRegion,
			len(group.ECVMs),
		)

		if len(group.ECVMs) == 0 {
			continue
		}

		// Iterate all the .ecVMs within each group and print out their attributes such as name, operationalStatus, status
		for _, ecVM := range group.ECVMs {
			// Extract unique name for each ECVM based on last hyphenated string
			nodeSuffix := ecVM.Name[strings.LastIndex(ecVM.Name, "-")+1:]
			nodeIp := ecVM.ECInstances[0].ServiceNw.IPStart

			// convert a UTC timestamp to local date time
			lastUpgradeTimeUtcTs := int64(ecVM.LastUpgradeTime)
			var lastUpgradeTime string
			if ecVM.LastUpgradeTime == 0 {
				lastUpgradeTime = "None"
			} else {
				lastUpgradeTime = time.Unix(lastUpgradeTimeUtcTs, 0).Local().Format("02-01-2006 15:04:05")
			}

			fmt.Printf(
				"\t - Id: %d\tSuffix: %s\tOpStatus: %s\tLastUpgrade: %s\t\tStatus: %s\tIP: %s\n",
				ecVM.ID,
				nodeSuffix,
				ecVM.OperationalStatus,
				lastUpgradeTime,
				ecVM.Status,
				nodeIp,
			)

			isInDeletingState := false
			for _, status := range ecVM.Status {
				if status == "DELETING" {
					isInDeletingState = true
				}
			}

			if ecVM.OperationalStatus == "INACTIVE" {
				currentTime := time.Now().Format("02/01/06 15:04:05 MST")
				fmt.Printf("⚠️ ECVM %s is not ACTIVE... Queued for deletion (%s)\n", ecVM.Name, currentTime)
				fmt.Printf("\t==Raw JSON Received ==> %s\n", getJsonOutputPath())
				if !isInDeletingState {
					err := EcVMDelete(ctx, service, group.ID, ecVM)
					if err != nil {
						fmt.Printf("Error deleting EC VM []: %v", ecVM.Name, err)
						return
					}
				} else {
					fmt.Printf("ECVM %s is already in DELETING state\n", ecVM.Name)
				}
			}
		}
		fmt.Println()
	}
}

func EcVMDelete(ctx context.Context, service *services.Service, ecGroupId int, ecVM ECVMs) error {
	var err error
	// Prompt for human confirmation before proceeding
	fmt.Printf("\n⚠️⚠️⚠️ !! CONFIRM DELETION !!\n\tDo you want to delete ECVM ID %v from Group %d? [y/N]: ", ecVM.Name, ecGroupId)
	if DRY_RUN_MODE {
		fmt.Printf("\n\t❇️ [DRY_RUN] Action: Delete | Path: %s\n", fmt.Sprintf("%s/%d/vm/%d", ecGroupEndpoint, ecGroupId, ecVM.ID))
		return nil
	} else {
		var response string
		fmt.Scanln(&response)

		// Default to No if empty response or anything other than 'y' or 'yes'
		response = strings.ToLower(strings.TrimSpace(response))
		if response != "y" && response != "yes" {
			fmt.Println("❌ Deletion cancelled by user")
			return nil
		}

		fmt.Println("✅🚮 Deletion confirmed by user, proceeding...")
		err = service.Client.Delete(ctx,
			fmt.Sprintf("%s/%d/vm/%d", ecGroupEndpoint, ecGroupId, ecVM.ID))

		// Adding in offset time purely
		sleepTime := time.Second * 30
		fmt.Printf("\n\t... Sleeping for %v secs to offset deletions ...", sleepTime.Seconds())
		time.Sleep(sleepTime)
	}
	return err
}

func EcGroupGetAllExtended(ctx context.Context, service *services.Service) ([]EcGroup, *[]byte, error) {
	var ecgroups []EcGroup
	var rawResponse []interface{}

	err := common.ReadAllPages(ctx, service.Client, ecGroupEndpoint, &rawResponse)
	if err != nil {
		return nil, nil, err
	}

	// Marshal the raw response to get the JSON bytes for debugging
	rawJSON, err := json.Marshal(rawResponse)
	if err != nil {
		return nil, nil, err
	}

	// Unmarshal into the structured type
	err = json.Unmarshal(rawJSON, &ecgroups)
	if err != nil {
		return nil, &rawJSON, err
	}

	return ecgroups, &rawJSON, nil
}

func getJsonOutputPath() string {
	if JSON_OUTPUT_PATH == nil {
		timestamp := time.Now().Unix()
		outputPath := fmt.Sprintf("%s/ecgroups_%d.json", JSON_PATH_PREFIX, timestamp)
		JSON_OUTPUT_PATH = &outputPath
		err = os.WriteFile(outputPath, *JSON_RAW_BYTES, 0644)
		if err != nil {
			log.Fatalf("Error writing JSON output to file: %v", err)
		}
		//fmt.Printf("\t>> First Init -- Raw JSON output written to: %s\n", outputPath)
	}
	return *JSON_OUTPUT_PATH
}
