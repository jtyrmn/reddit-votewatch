package util

import (
	"fmt"
	"log"
	"os"
	"strconv"
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

//get an integer
func GetEnvInt(str string) int {
	v := GetEnv(str)

	i, err := strconv.ParseInt(v, 10, 32)
	if err != nil {
		log.Fatalf("cannot parse environment variable %s=%s:%s halting executing...\n", str, v, err.Error())
	}

	return int(i)
}