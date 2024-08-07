package env

import (
	"os"
	"strconv"
	"time"

	v1 "github.com/openshift/api/config/v1"
)

const (
	envExistingCrName         = "TEST_EXISTING_CR_NAME"
	envExistingCrNamespace    = "TEST_EXISTING_CR_NAMESPACE"
	envSkipUpdateSspTests     = "SKIP_UPDATE_SSP_TESTS"
	envSkipCleanupAfterTests  = "SKIP_CLEANUP_AFTER_TESTS"
	envTimeout                = "TIMEOUT_MINUTES"
	envShortTimeout           = "SHORT_TIMEOUT_MINUTES"
	envTopologyMode           = "TOPOLOGY_MODE"
	envIsUpgradeLane          = "IS_UPGRADE_LANE"
	envSspDeploymentName      = "SSP_DEPLOYMENT_NAME"
	envSspDeploymentNamespace = "SSP_DEPLOYMENT_NAMESPACE"
	envSspWebhookServiceName  = "SSP_WEBHOOK_SERVICE_NAME"
	envSkipDeployedByHCO      = "SKIP_SSP_DEPLOYED_BY_HCO"
)

const (
	defaultTimeout      = 10 * time.Minute
	defaultShortTimeout = 1 * time.Minute
)

func ExistingCrName() string {
	return os.Getenv(envExistingCrName)
}

func ExistingCrNamespace() string {
	return os.Getenv(envExistingCrNamespace)
}

func IsUpgradeLane() bool {
	return getBoolEnv(envIsUpgradeLane)
}

func SkipUpdateSspTests() bool {
	return getBoolEnv(envSkipUpdateSspTests)
}

func ShouldSkipCleanupAfterTests() bool {
	return getBoolEnv(envSkipCleanupAfterTests)
}

func IsDeployedByHCO() bool {
	return getBoolEnv(envSkipDeployedByHCO)
}

var timeout time.Duration

func Timeout() time.Duration {
	if timeout == 0 {
		if intValue, exists := getIntEnv(envTimeout); exists {
			timeout = time.Minute * time.Duration(intValue)
		} else {
			timeout = defaultTimeout
		}
	}
	return timeout
}

var shortTimeout time.Duration

func ShortTimeout() time.Duration {
	if shortTimeout == 0 {
		if intValue, exists := getIntEnv(envShortTimeout); exists {
			shortTimeout = time.Minute * time.Duration(intValue)
		} else {
			shortTimeout = defaultShortTimeout
		}
	}
	return shortTimeout
}

// TopologyMode returns ("", false) if an env var is not set or (X, true) if it is set
func TopologyMode() (v1.TopologyMode, bool) {
	envVal := os.Getenv(envTopologyMode)
	if envVal == string(v1.SingleReplicaTopologyMode) {
		return v1.SingleReplicaTopologyMode, true
	} else if envVal == string(v1.HighlyAvailableTopologyMode) {
		return v1.HighlyAvailableTopologyMode, true
	}
	return "", false
}

func SspDeploymentName() string {
	return os.Getenv(envSspDeploymentName)
}

func SspDeploymentNamespace() string {
	return os.Getenv(envSspDeploymentNamespace)
}

func SspWebhookServiceName() string {
	return os.Getenv(envSspWebhookServiceName)
}

func getBoolEnv(envName string) bool {
	envVal := os.Getenv(envName)
	if envVal == "" {
		return false
	}
	val, err := strconv.ParseBool(envVal)
	if err != nil {
		return false
	}
	return val
}

// getIntEnv returns (0, false) if an env var is not set or (X, true) if it is set
func getIntEnv(envName string) (int, bool) {
	envVal := os.Getenv(envName)
	if envVal == "" {
		return 0, false
	} else {
		val, err := strconv.ParseInt(envVal, 10, 32)
		if err != nil {
			panic(err)
		}
		return int(val), true
	}
}
