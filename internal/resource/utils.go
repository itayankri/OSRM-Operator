package resource

import (
	"fmt"
	"strings"
)

func serviceToEnvVariable(serviceName string) string {
	return fmt.Sprintf("%s_SERVICE_HOST", strings.ReplaceAll(strings.ToUpper(serviceName), "-", "_"))
}
