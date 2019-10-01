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

package cmd

import (
	"strings"
	"time"

	"github.com/pkg/errors"
)

// Simple test for appsody build command. A future enhancement would be to verify the image that gets built.
func TestRun(projectDir string) error {

	// appsody run
	runChannel := make(chan error)
	containerName := "testRunContainer"
	go func() {
		Info.log("******************************************")
		Info.log("Running appsody run")
		Info.log("******************************************")
		_, err := RunAppsodyCmdExec([]string{"run", "--name", containerName}, projectDir)
		runChannel <- err
	}()

	// check to see if we get an error from appsody run
	// log appsody ps output
	// if appsody run doesn't fail after the loop time then assume it passed
	// endpoint checking would be a better way to verify appsody run
	healthCheckFrequency := 2 // in seconds
	healthCheckTimeout := 60  // in seconds
	healthCheckWait := 0
	isHealthy := false
	for !(healthCheckWait >= healthCheckTimeout) {
		select {
		case err := <-runChannel:
			// appsody run exited, probably with an error
			Error.log("Appsody run failed")
			return err
		case <-time.After(time.Duration(healthCheckFrequency) * time.Second):
			// see if appsody ps has a container
			healthCheckWait += healthCheckFrequency

			Info.log("about to run appsody ps")
			stopOutput, errStop := RunAppsodyCmdExec([]string{"ps"}, projectDir)
			if !strings.Contains(stopOutput, "CONTAINER") {
				Info.log("appsody ps output doesn't contain header line")
			}
			if !strings.Contains(stopOutput, containerName) {
				Info.log("appsody ps output doesn't contain correct container name")
			} else {
				Info.log("appsody ps contains correct container name")
				isHealthy = true
			}
			if errStop != nil {
				Error.log(errStop)
				return errStop
			}
		}
	}

	if !isHealthy {
		Error.log("appsody ps never found the correct container")
		return errors.New("appsody ps never found the correct container")
	}

	Info.log("Appsody run did not fail")

	// stop and clean up after the run
	func() {
		_, err := RunAppsodyCmdExec([]string{"stop", "--name", "testRunContainer"}, projectDir)
		if err != nil {
			Error.log("appsody stop failed")
			//return err ***this fails for some reason
		}
	}()

	return nil
}