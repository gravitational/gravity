package defaults

import (
	"time"

	"github.com/gravitational/gravity/lib/constants"
)

var (
	// LicenseOutputFormat represents the output format of the show license command
	LicenseOutputFormat = constants.EncodingPEM
	// TrustedClusterReconnectInterval is how long to wait before recreating
	// trusted cluster
	TrustedClusterReconnectInterval = 15 * time.Second
)
