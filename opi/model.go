package opi

import (
	"fmt"
)

const (
	RunningState            = "RUNNING"
	PendingState            = "CLAIMED"
	ErrorState              = "UNCLAIMED"
	CrashedState            = "CRASHED"
	UnknownState            = "UNKNOWN"
	InsufficientMemoryError = "Insufficient resources: memory"
)

type LRPIdentifier struct {
	GUID, Version string
}

func (i *LRPIdentifier) ProcessGUID() string {
	return fmt.Sprintf("%s-%s", i.GUID, i.Version)
}

// An LRP, or long-running-process, is a stateless process
// where the scheduler should attempt to keep N copies running,
// killing and recreating as needed to maintain that guarantee.
type LRP struct {
	LRPIdentifier
	ProcessType            string
	AppName                string
	AppGUID                string
	OrgName                string
	OrgGUID                string
	SpaceName              string
	SpaceGUID              string
	Image                  string
	Command                []string
	PrivateRegistry        *PrivateRegistry
	Env                    map[string]string
	Health                 Healtcheck
	Ports                  []int32
	TargetInstances        int
	RunningInstances       int
	MemoryMB               int64
	DiskMB                 int64
	RunsAsRoot             bool
	CPUWeight              uint8
	VolumeMounts           []VolumeMount
	LRP                    string
	AppURIs                string
	LastUpdated            string
	UserDefinedAnnotations map[string]string
}

type PrivateRegistry struct {
	Server   string `json:"server"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type VolumeMount struct {
	MountPath string
	ClaimName string
}

type Instance struct {
	Index          int
	Since          int64
	State          string
	PlacementError string
}

type Healtcheck struct {
	Type      string
	Port      int32
	Endpoint  string
	TimeoutMs uint
}

// A Task is a one-off process that is run exactly once and returns a
// result.
type Task struct {
	GUID               string            `json:"guid"`
	Name               string            `json:"name"`
	Image              string            `json:"image"`
	CompletionCallback string            `json:"completionCallback"`
	PrivateRegistry    *PrivateRegistry  `json:"privateRegistry,omitempty"`
	Env                map[string]string `json:"env"`
	Command            []string          `json:"command"`
	AppName            string            `json:"appName"`
	AppGUID            string            `json:"appGUID"`
	OrgName            string            `json:"orgName"`
	OrgGUID            string            `json:"orgGUID"`
	SpaceName          string            `json:"spaceName"`
	SpaceGUID          string            `json:"spaceGUID"`
	MemoryMB           int64             `json:"memoryMB"`
	DiskMB             int64             `json:"diskMB"`
	CPUWeight          uint8             `json:"cpuWeight"`
}

type StagingTask struct {
	*Task
	DownloaderImage string
	UploaderImage   string
	ExecutorImage   string
}
