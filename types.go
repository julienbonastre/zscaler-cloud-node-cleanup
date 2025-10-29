package main

import "github.com/zscaler/zscaler-sdk-go/v3/zscaler/zcon/services/common"

type EcGroup struct {
	AwsRegion           string   `json:"awsRegion"`
	ID                  int      `json:"id,omitempty"`
	Name                string   `json:"name,omitempty"`
	Description         string   `json:"desc,omitempty"`
	DeployType          string   `json:"deployType,omitempty"`
	Status              []string `json:"status,omitempty"`
	Platform            string   `json:"platform,omitempty"`
	AWSAvailabilityZone string   `json:"awsAvailabilityZone,omitempty"`
	//AzureAvailabilityZone string                 `json:"azureAvailabilityZone,omitempty"`
	MaxEcCount int                    `json:"maxEcCount,omitempty"`
	TunnelMode string                 `json:"tunnelMode,omitempty"`
	Location   *common.GeneralPurpose `json:"location,omitempty"`
	//ProvTemplate          *common.GeneralPurpose `json:"provTemplate,omitempty"`
	ECVMs []ECVMs `json:"ecVMs,omitempty"`
}

type ECVMs struct {
	OperationalStatus string   `json:"operationalStatus"`
	Status            []string `json:"status"`
	ID                int      `json:"id,omitempty"`
	Name              string   `json:"name,omitempty"`
	FormFactor        string   `json:"formFactor,omitempty"`
	//CityGeoId        int           `json:"cityGeoId,omitempty"`
	//NATIP            string        `json:"natIp,omitempty"`
	//ZiaGateway       string        `json:"ziaGateway,omitempty"`
	//ZpaBroker        string        `json:"zpaBroker,omitempty"`
	BuildVersion     string `json:"buildVersion,omitempty"`
	LastUpgradeTime  int    `json:"lastUpgradeTime,omitempty"`
	UpgradeStatus    int    `json:"upgradeStatus,omitempty"`
	UpgradeStartTime int    `json:"upgradeStartTime,omitempty"`
	UpgradeEndTime   int    `json:"upgradeEndTime,omitempty"`
	//ManagementNw     *ManagementNw `json:"managementNw,omitempty"`
	ECInstances []common.ECInstances `json:"ecInstances,omitempty"`
}
