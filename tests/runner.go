package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/manifoldco/promptui"
)

var testGroups = map[string][]string{
	"log":       {"fields", "level", "mode", "logger", "options", "pretty", "json", "benchmark", "allLog"},
	"config":    {"loadConfig", "fromMap", "get", "getString", "getInt", "getInt64", "getBool", "getFloat64", "getDuration", "getOrDefault", "getStringOrDefault", "getIntOrDefault", "getBoolOrDefault", "has", "unmarshalKey", "set", "allConfig"},
	"converter": {"toString", "toInt", "toInt64", "toBool", "toDuration", "allConverter"},
	"json":      {"loadJson", "unmarshal", "unmarshalInto", "notFound", "invalidJSON", "allJson"},
}

var testPatterns = map[string]string{
	"allLog":             "TestLogger_WithFields|TestLogger_LevelFiltering|TestLogger_Concurrency|TestLogger_Info_Output",
	"fields":             "TestLogger_WithFields",
	"level":              "TestLogger_LevelFiltering",
	"mode":               "TestLogger_Info_Output|TestLogger_WithFields",
	"logger":             "TestLogger_Info_Output|TestLogger_Concurrency",
	"options":            "TestLogger",
	"pretty":             "TestLogger_LevelFiltering|TestLogger_Concurrency",
	"json":               "TestLogger_Info_Output|TestLogger_WithFields",
	"benchmark":          "BenchmarkLogger_Info",
	"allConfig":          "TestConfig_Load|TestConfig_FromMap|TestConfig_Get|TestConfig_GetString|TestConfig_GetInt|TestConfig_GetInt64|TestConfig_GetBool|TestConfig_GetFloat64|TestConfig_GetDuration|TestConfig_GetOrDefault|TestConfig_GetStringOrDefault|TestConfig_GetIntOrDefault|TestConfig_GetBoolOrDefault|TestConfig_Has|TestConfig_UnmarshalKey|TestConfig_Set",
	"loadConfig":         "TestConfig_Load",
	"fromMap":            "TestConfig_FromMap",
	"get":                "TestConfig_Get",
	"getString":          "TestConfig_GetString",
	"getInt":             "TestConfig_GetInt",
	"getInt64":           "TestConfig_GetInt64",
	"getBool":            "TestConfig_GetBool",
	"getFloat64":         "TestConfig_GetFloat64",
	"getDuration":        "TestConfig_GetDuration",
	"getOrDefault":       "TestConfig_GetOrDefault",
	"getStringOrDefault": "TestConfig_GetStringOrDefault",
	"getIntOrDefault":    "TestConfig_GetIntOrDefault",
	"getBoolOrDefault":   "TestConfig_GetBoolOrDefault",
	"has":                "TestConfig_Has",
	"unmarshalKey":       "TestConfig_UnmarshalKey",
	"set":                "TestConfig_Set",
	"allConverter":       "TestConverter_ToString|TestConverter_ToInt|TestConverter_ToInt64|TestConverter_ToBool|TestConverter_ToDuration",
	"toString":           "TestConverter_ToString",
	"toInt":              "TestConverter_ToInt",
	"toInt64":            "TestConverter_ToInt64",
	"toBool":             "TestConverter_ToBool",
	"toDuration":         "TestConverter_ToDuration",
	"loadJson":           "TestJson_Load",
	"notFound":           "TestJson_Load_NotFound",
	"invalidJSON":        "TestJson_Load_InvalidJSON",
	"unmarshal":          "TestJson_Unmarshal",
	"unmarshalInto":      "TestJson_UnmarshalInto",
	"allJson":            "TestJson_Load|TestJson_Unmarshal|TestJson_UnmarshalInto|TestJson_Load_NotFound|TestJson_Load_InvalidJSON",
}

func main() {
	consumerType := flag.String("consumer-type", "", "Specify a test to run directly (e.g., log/fields, or fields)")
	flag.Parse()

	if *consumerType != "" {
		runTestByName(*consumerType)
		return
	}

	// Step 1: Select folder (e.g., log)
	folderNames := make([]string, 0, len(testGroups))
	for k := range testGroups {
		folderNames = append(folderNames, k)
	}
	sort.Strings(folderNames)

	folderPrompt := promptui.Select{
		Label: "Select folder",
		Items: folderNames,
	}
	folderIndex, _, err := folderPrompt.Run()
	if err != nil {
		fmt.Printf("Prompt failed: %v\n", err)
		os.Exit(1)
	}
	folder := folderNames[folderIndex]

	// Step 2: Select test within folder
	testNames := testGroups[folder]
	testPrompt := promptui.Select{
		Label: "Select test to run",
		Items: testNames,
	}
	_, test, err := testPrompt.Run()
	if err != nil {
		fmt.Printf("Prompt failed: %v\n", err)
		os.Exit(1)
	}

	runTestByName(folder + "/" + test)
}

func runTestByName(name string) {
	// Support "log/fields" or "fields" (bare name)
	key := name
	if !strings.Contains(name, "/") {
		key = "log/" + name
	}
	// Extract test name (part after /)
	parts := strings.Split(key, "/")
	testName := parts[len(parts)-1]

	pattern, ok := testPatterns[testName]
	if !ok {
		fmt.Printf("Invalid test: %s\n", name)
		os.Exit(1)
	}

	runLoggerTests(pattern)
}

func runLoggerTests(pattern string) {
	dir, err := filepath.Abs(".")
	if err != nil {
		fmt.Printf("Failed to get working directory: %v\n", err)
		os.Exit(1)
	}

	var args []string
	if strings.HasPrefix(pattern, "Benchmark") {
		args = []string{"test", "./tests", "-bench", "^" + pattern + "$", "-benchmem"}
	} else {
		args = []string{"test", "./tests", "-v", "-run", pattern}
	}

	cmd := exec.Command("go", args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		os.Exit(1)
	}
}
