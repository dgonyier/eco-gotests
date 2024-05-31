package assistedconfig

import (
	"log"

	"github.com/openshift-kni/eco-gotests/tests/internal/config"
)

// AssistedConfig type contains assisted installer configuration.
type AssistedConfig struct {
	*config.GeneralConfig
}

// NewAssistedConfig returns instance of AssistedConfig type.
func NewAssistedConfig() *AssistedConfig {
	log.Print("Creating new AssistedConfig struct")

	var assisetdConfig AssistedConfig
	assisetdConfig.GeneralConfig = config.NewConfig()

	return &assisetdConfig
}
