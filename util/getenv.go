package util

import (
	"fmt"
	"log"
	"os"
)

//get environment variable
func GetEnv(str string) string {
	v, exists := os.LookupEnv(str)
	if !exists {
		log.Fatalf("cannot find environment variable \"%s\": halting execution...\n", str)
	}

	return v
}

//equivelant to getEnv except doesn't cause an error and substitutes a default value (def)
func GetEnvDefault(str string, def string) string {
	var v string
	v, exists := os.LookupEnv(str)
	if !exists {
		fmt.Printf("warning: env variable %s not found, defaulting to \"%s\"...\n", str, def)
		return def
	}

	return v
}
