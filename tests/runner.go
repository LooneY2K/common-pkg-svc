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
	"log": {"fields", "level", "mode", "logger", "options", "pretty", "json", "benchmark"},
}

var testPatterns = map[string]string{
	"fields":    "TestLogger_WithFields",
	"level":     "TestLogger_LevelFiltering",
	"mode":      "TestLogger_Info_Output|TestLogger_WithFields",
	"logger":    "TestLogger_Info_Output|TestLogger_Concurrency",
	"options":   "TestLogger",
	"pretty":    "TestLogger_LevelFiltering|TestLogger_Concurrency",
	"json":      "TestLogger_Info_Output|TestLogger_WithFields",
	"benchmark": "BenchmarkLogger_Info",
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
