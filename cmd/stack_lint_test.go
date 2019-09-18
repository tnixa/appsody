// Copyright © 2019 IBM Corporation and others.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd_test

import (
	"log"
	"os"
	"path/filepath"
	"testing"

	"github.com/appsody/appsody/cmd/cmdtest"
)

func TestLintWithValidStack(t *testing.T) {
	currentDir, _ := os.Getwd()
	testStackPath := filepath.Join(currentDir, "testdata", "test-stack")
	args := []string{"stack", "lint"}

	_, err := cmdtest.RunAppsodyCmdExec(args, testStackPath)

	if err != nil { //Lint check should pass, if not fail the test
		t.Fatal(err)
	}
}

func TestLintWithMissingConfig(t *testing.T) {
	currentDir, _ := os.Getwd()
	testStackPath := filepath.Join(currentDir, "testdata", "test-stack")
	args := []string{"stack", "lint"}
	removeConf := filepath.Join(testStackPath, "image", "config")
	removeArray := []string{removeConf, filepath.Join(removeConf, "app-deploy.yaml")}

	os.RemoveAll(removeConf)

	_, err := cmdtest.RunAppsodyCmdExec(args, testStackPath)

	if err != nil { //Lint check should pass, if not fail the test
		t.Fatal(err)
	}

	RestoreSampleStack(removeArray)
}

func TestLintWithMissingProject(t *testing.T) {
	currentDir, _ := os.Getwd()
	testStackPath := filepath.Join(currentDir, "testdata", "test-stack")
	args := []string{"stack", "lint"}
	removeProj := filepath.Join(testStackPath, "image", "project")
	removeArray := []string{removeProj, filepath.Join(removeProj, "Dockerfile")}

	osErr := os.RemoveAll(removeProj)

	if osErr != nil {
		t.Fatal(osErr)
	}

	_, err := cmdtest.RunAppsodyCmdExec(args, testStackPath)

	if err != nil { //Lint check should pass, if not fail the test
		t.Fatal(err)
	}

	RestoreSampleStack(removeArray)
}

func TestLintWithMissingFile(t *testing.T) {
	currentDir, _ := os.Getwd()
	testStackPath := filepath.Join(currentDir, "testdata", "test-stack")
	args := []string{"stack", "lint"}
	removeReadme := filepath.Join(testStackPath, "README.md")
	removeArray := []string{removeReadme}

	osErr := os.RemoveAll(removeReadme)
	if osErr != nil {
		t.Fatal(osErr)
	}

	_, err := cmdtest.RunAppsodyCmdExec(args, testStackPath)

	if err == nil { //Lint check should fail, if not fail the test
		t.Fatal(err)
	}

	RestoreSampleStack(removeArray)
}

func RestoreSampleStack(fixStack []string) {
	currentDir, _ := os.Getwd()
	testStackPath := filepath.Join(currentDir, "testdata", "test-stack")
	for _, missingContent := range fixStack {
		if missingContent == filepath.Join(testStackPath, "image/config") || missingContent == filepath.Join(testStackPath, "image/project") {
			osErr := os.Mkdir(missingContent, os.ModePerm)
			if osErr != nil {
				log.Println(osErr)
			}
		} else {
			_, osErr := os.Create(missingContent)
			if osErr != nil {
				log.Println(osErr)
			}
		}
	}
}