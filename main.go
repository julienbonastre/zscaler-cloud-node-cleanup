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
)

var (
	DRY_RUN_MODE = true
	err          error
)

//go:embed assets/*
var assets embed.FS

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

	ccgroups, err := EcGroupGetAllExtended(ctx, service)
	if err != nil {
		log.Fatalf("Error listing EC Groups: %v", err)
	}

	fmt.Printf("EC Groups: %d\n", len(ccgroups))

	// Extract out the top level list of all the Zscaler Cloud Connector Groups
	for _, group := range ccgroups {
		fmt.Printf(
			"⊳ Group (%d) ==> %s [%s] with %d ECVMs\n",
			group.ID,
			group.Name,
			group.AwsRegion,
			len(group.ECVMs),
		)
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
				fmt.Printf("⚠️ ECVM %s is not ACTIVE... Queued for deletion\n", ecVM.Name)

				if !isInDeletingState {
					err := EcVMDelete(ctx, service, group.ID, ecVM.ID)
					if err != nil {
						fmt.Printf("Error deleting EC VM []: %v", ecVM.Name, err)
						return
					}
				} else {
					fmt.Printf("ECVM %s is already in DELETING state\n", ecVM.Name)
				}
			}
		}
	}
}

func EcVMDelete(ctx context.Context, service *services.Service, ecGroupId int, ecVMID int) error {
	if DRY_RUN_MODE {
		fmt.Printf("[DRY_RUN] Action: Delete | Path: %s\n", fmt.Sprintf("%s/%d/vm/%d", ecGroupEndpoint, ecGroupId, ecVMID))
		return nil
	}
	err := service.Client.Delete(ctx, fmt.Sprintf("%s/%d/vm/%d", ecGroupEndpoint, ecGroupId, ecVMID))
	return err
}

func EcGroupGetAllExtended(ctx context.Context, service *services.Service) ([]EcGroup, error) {
	var ecgroups []EcGroup
	err := common.ReadAllPages(ctx, service.Client, ecGroupEndpoint, &ecgroups)
	return ecgroups, err
}
