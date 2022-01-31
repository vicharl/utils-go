// Package vergen generate version info with go generate
package vergen

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"
)

const (
	DefaultPkgName     = "version"
	DefaultDirtyString = "+"
	DefaultTimeout     = 15 * time.Second
)

// DirtyString is a string appended to the VgVersion constant if you have
// uncommitted changes in your current repo.
var DirtyString = DefaultDirtyString

// IgnoreFiles is an array of filenames that vergen should ignore when
// trying to detect uncommitted changes. By default it contains only "version.go"
// which if it is the only file changed doesn't affect the runtime behaviour
// of your software.
var IgnoreFiles = []string{"version.go"}

// Timeout is of type time.Duration and is the amount of time vergen will
// wait for each git command it runs to complete before killing it.
var Timeout = DefaultTimeout

type versionData struct {
	describeTags string
	commit       string
	dirty        bool
}

// Create will create a file named version.go in the directory you ran
// 'go generate', that will contain three constants:
//
// VgVersion: The version of your repo as given by 'git describe --tags'
// plus the DirtyString variable if you have uncommitted changes.
//
// VgHash: The SHA1 hash of your current commit.
//
// VgClean: indicates whether your build is clean or it includes uncommitted changes.
func Create() error {
	return CreateFile(DefaultPkgName + "/" + DefaultPkgName + ".go")
}

// CreateFile will work as Create() but instead of writing version.go in the
// directory you ran 'go generate' in, it will write filename instead (which
// may include a path).
func CreateFile(filename string) error {
	// Get version
	version, err := runGitSingleLineReturn("git", "describe", "--tags")
	if err != nil {
		version = "0.1.0"
		//return errors.New("Could not run 'git describe --tags'. " +
		//	"Are there any tags in your repo? Error: " + err.Error())
	}

	// Get SHA1
	hash, err := runGitSingleLineReturn("git", "rev-parse", "HEAD")
	if err != nil {
		return errors.New("Could not run 'git rev-parse HEAD' " +
			"to get commit hash. Error: " + err.Error())
	}

	// Get uncommitted changes, split them by line
	diffIndex, err := runGitSingleLineReturn("git", "diff-index", "HEAD")
	if err != nil {
		errors.New("Could not run 'git diff-index HEAD' " +
			"to detect uncommitted changes. Error: " + err.Error())
	}
	var diffIndexLines []string
	if strings.TrimSpace(diffIndex) != "" { // Because direct assignment will give us []string{""}
		diffIndexLines = strings.Split(strings.TrimSpace(diffIndex), "\n")
	}

	uncommittedChanges := false
UNCOMMITTED:
	for _, v := range diffIndexLines { // For each uncommitted, changed file
		matches := 0
		for _, v2 := range IgnoreFiles { // Check against each of blacklisted files
			if strings.Contains(v, v2) {
				matches++
			}
		}
		if matches == 0 { // If none matched, this file has uncommitted changes, build is dirty
			uncommittedChanges = true
			break UNCOMMITTED
		}
	}
	if uncommittedChanges {
		version += DirtyString
	}

	log.Printf("Setting VgVersion to: %v\n", version)
	log.Printf("Setting VgHash to: %v\n", hash)
	log.Printf("Setting VgClean to: %v\n", !uncommittedChanges)

	err = writeVersionFile(filename, versionData{version, hash, uncommittedChanges})
	return err
}

// Run a command and return the results and possible errors.
func runGitSingleLineReturn(command string, args ...string) (string, error) {
	cmd := exec.Command(command, args...)

	timer := time.AfterFunc(Timeout, func() { cmd.Process.Kill() })
	out, err := cmd.CombinedOutput()
	timer.Stop()
	if err != nil {
		return "", err
	}

	result := strings.TrimSpace(string(out))
	return result, nil
}

// Write the version file.
func writeVersionFile(filename string, data versionData) error {
	if _, err := os.Stat(DefaultPkgName); os.IsNotExist(err) {
		os.Mkdir(DefaultPkgName, os.ModePerm)
	}
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	out := fmt.Sprintf(`package %s
// auto generated by github.com/Akagi201/utils-go/vergen
const (
	VgVersion   = "%s"
	VgHash      = "%s"
	VgClean     = %v
)
`, DefaultPkgName, data.describeTags, data.commit, !data.dirty)

	_, err = file.Write([]byte(out))
	return err
}
