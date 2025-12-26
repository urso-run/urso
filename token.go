package urso

import "os"

const (
	// EnvVarRegisterToken is the default environment variable for the GitHub runner registration token.
	EnvVarRegisterToken = "GITHUB_REGISTER_TOKEN" //gosec:disable G101 -- This is a false positive
	// EnvVarRemoveToken is the default environment variable for the GitHub runner removal token.
	EnvVarRemoveToken = "GITHUB_REMOVE_TOKEN" //gosec:disable G101 -- This is a false positive
)

// ResolveToken determines the correct token to use based on a defined order of precedence:
// 1. A non-empty value provided from a command-line flag.
// 2. A non-empty value from a specified environment variable.
// If both are empty, an empty string is returned.
func ResolveToken(flagValue, envVarName string) string {
	if flagValue != "" {
		return flagValue
	}
	if envVarValue := os.Getenv(envVarName); envVarValue != "" {
		return envVarValue
	}
	return ""
}
