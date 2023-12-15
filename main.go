package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// FindReplace is an object represent in a specified yaml config
type FindReplace struct {
	Find    string `yaml:"find"`
	Replace string `yaml:"replace"`
}

func main() {
	// Parse command-line arguments
	goModPath := flag.String("gomod", "go.mod.test", "Path to the go.mod file")
	goModConfigPath := flag.String("config", "replace.yaml", "Path to a config containing find and replace")
	clean := flag.Bool("clean", false, "Remove all replace cmds")
	flag.Parse()

	if err := deleteLinesWithReplace(*goModPath); err != nil {
		log.Fatal(err)
	}

	// If clean, our job here is done
	if *clean {
		return
	}

	// Read the find replace config
	find, err := readYamlConfig(*goModConfigPath)
	if err != nil {
		log.Fatal(err)
	}

	// Scan go mod for any matching modules
	replace, err := findMatchesInFile(*goModPath, find)
	if err != nil {
		log.Fatal(err)
	}

	// Validate replace mods exist
	if err = validateLocalReposExist(replace); err != nil {
		log.Fatal(err)
	}

	// Append replace statements to go.mod
	if err = appendModReplace(*goModPath, replace); err != nil {
		log.Fatal(err)
	}
}

func readYamlConfig(filePath string) ([]FindReplace, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	byteValue, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	var findReplaces []FindReplace
	err = yaml.Unmarshal(byteValue, &findReplaces)
	if err != nil {
		return nil, err
	}

	return findReplaces, nil
}

func findMatchesInFile(filePath string, find []FindReplace) ([]FindReplace, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var found []FindReplace

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()

		for _, cmd := range find {
			if strings.Contains(line, cmd.Find) {
				found = append(found, cmd)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return found, nil
}

func validateLocalReposExist(replace []FindReplace) error {
	var missing []string

	for _, cmd := range replace {
		exists, err := dirExists(cmd.Replace)
		if err != nil {
			missing = append(missing, err.Error())
			continue
		}

		if !exists {
			missing = append(missing, cmd.Replace)
		}
	}

	if len(missing) != 0 {
		combinedMissingStr := strings.Join(missing, "\n")
		return fmt.Errorf("replace module error(s) or missing:\n%s", combinedMissingStr)
	}

	return nil
}

// dirExists checks if a given path exists and is a directory.
func dirExists(path string) (bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			// The path does not exist
			return false, nil
		}
		// There was some other error accessing the path
		return false, err
	}
	// The path exists; check if it's a directory
	return info.IsDir(), nil
}

func appendModReplace(goModPath string, replace []FindReplace) error {
	// Read the original file content
	originalContent, err := os.ReadFile(goModPath)
	if err != nil {
		return err
	}

	// Create a temporary file
	tempFile, err := os.CreateTemp("", "go.mod.temp")
	if err != nil {
		return err
	}
	defer tempFile.Close()
	defer os.Remove(tempFile.Name()) // Clean up

	// Write the original content to the temporary file
	_, err = tempFile.Write(originalContent)
	if err != nil {
		return err
	}

	// Append the new lines
	for _, cmd := range replace {
		_, err = tempFile.WriteString(fmt.Sprintf("replace %s => %s\n", cmd.Find, cmd.Replace))
		if err != nil {
			return err
		}
	}

	// Close the temporary file
	if err := tempFile.Close(); err != nil {
		return err
	}

	// Replace the original file with the temporary file
	return os.Rename(tempFile.Name(), goModPath)
}

func deleteLinesWithReplace(filePath string) error {
	// Open the original file
	originalFile, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer originalFile.Close()

	// Create a temporary file
	tempFile, err := os.CreateTemp("", "go.mod.temp")
	if err != nil {
		return err
	}
	defer tempFile.Close()
	defer os.Remove(tempFile.Name()) // Cleanup in case of error

	// Scanner to read the original file
	scanner := bufio.NewScanner(originalFile)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "replace") {
			if _, err := tempFile.WriteString(line + "\n"); err != nil {
				return err
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	// Close files to ensure all data is written
	if err := originalFile.Close(); err != nil {
		return err
	}
	if err := tempFile.Close(); err != nil {
		return err
	}

	// Replace the original file with the temp file
	return os.Rename(tempFile.Name(), filePath)
}
