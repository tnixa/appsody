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
	"errors"
	"fmt"
	"os"
	"os/signal"
	"os/user"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"syscall"
	"time"

	flag "github.com/spf13/pflag"

	"github.com/spf13/cobra"
)

var containerName string
var depsVolumeName string
var ports []string
var publishAllPorts bool
var dockerNetwork string

var commonFlags *flag.FlagSet

func addDevCommonFlags(cmd *cobra.Command) {
	if commonFlags == nil {
		commonFlags = flag.NewFlagSet("", flag.ContinueOnError)
		curDir, err := os.Getwd()
		if err != nil {
			Error.log("Error getting current directory ", err)
			os.Exit(1)
		}
		defaultName := filepath.Base(curDir) + "-dev"
		defaultDepsVolume := filepath.Base(curDir) + "-deps"
		commonFlags.StringVar(&dockerNetwork, "network", "", "Specify the network for docker to use.")
		commonFlags.StringVar(&containerName, "name", defaultName, "Assign a name to your development container.")
		commonFlags.StringVar(&depsVolumeName, "deps-volume", defaultDepsVolume, "Docker volume to use for dependencies. Mounts to APPSODY_DEPS dir.")
		commonFlags.StringArrayVarP(&ports, "publish", "p", nil, "Publish the container's ports to the host. The stack's exposed ports will always be published, but you can publish addition ports or override the host ports with this option.")
		commonFlags.BoolVarP(&publishAllPorts, "publish-all", "P", false, "Publish all exposed ports to random ports")
	}
	cmd.PersistentFlags().AddFlagSet(commonFlags)
}

func commonCmd(cmd *cobra.Command, args []string, mode string) {

	err := CheckPrereqs()
	if err != nil {
		Warning.logf("Failed to check prerequisites: %v\n", err)
	}
	projectConfig := getProjectConfig()
	projectDir := getProjectDir()
	platformDefinition := projectConfig.Platform
	Debug.log("Stack image: ", platformDefinition)
	Debug.log("Project directory: ", projectDir)

	var cmdName string
	var cmdArgs []string
	dockerPullImage(platformDefinition)

	volumeMaps := getVolumeArgs()
	// Mount the APPSODY_DEPS cache volume if it exists
	depsEnvVar := getEnvVar("APPSODY_DEPS")
	if depsEnvVar != "" {
		depsMount := depsVolumeName + ":" + depsEnvVar
		Debug.log("Adding dependency cache to volume mounts: ", depsMount)
		volumeMaps = append(volumeMaps, "-v", depsMount)
	}

	// Mount the controller
	destController := os.Getenv("APPSODY_MOUNT_CONTROLLER")
	if destController != "" {
		Debug.log("Overriding appsody-controller mount with APPSODY_MOUNT_CONTROLLER env variable: ", destController)
	} else {
		// Check to see if the appsody-controller exists in the Home dir
		destController = filepath.Join(getHome(), "appsody-controller")
		Debug.log("Attempting to load the controller from ", destController)
		if _, err := os.Stat(destController); os.IsNotExist(err) {
			// it does not exist, so copy it from the executable dir
			//Retrieving the path of the binaries appsody and appsody-controller
			Debug.log("Didn't find the controller in .appsody - copying from the binary directory...")
			executable, _ := os.Executable()
			binaryLocation, err := filepath.Abs(filepath.Dir(executable))
			Debug.log("Binary location ", binaryLocation)
			if err != nil {
				Error.log("Fatal error - can't retrieve the binary path... exiting.")
				os.Exit(1)
			}
			//Construct the appsody-controller mount
			sourceController := filepath.Join(binaryLocation, "appsody-controller")
			if dryrun {
				Info.logf("Dry Run - Skipping copy of controller binary from %s to %s", sourceController, destController)
			} else {
				Debug.log("Attempting to copy the source controller from: ", sourceController)
				//Copy the controller from the binary location to $HOME/.appsody
				copyError := CopyFile(sourceController, destController)
				if copyError != nil {
					Error.log("Cannot retrieve controller - exiting: ", copyError)
					os.Exit(1)
				}
				// Making the controller executable in case CopyFile loses permissions
				chmodErr := os.Chmod(destController, 0755)
				if chmodErr != nil {
					Error.log("Cannot make the controller  executable - exiting: ", chmodErr)
					os.Exit(1)
				}
			}
		}
	}
	controllerMount := destController + ":/appsody/appsody-controller"
	Debug.log("Adding controller to volume mounts: ", controllerMount)
	volumeMaps = append(volumeMaps, "-v", controllerMount)

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		dockerStop(containerName)
		//dockerRemove(containerName) is not needed due to --rm flag
		os.Exit(1)
	}()
	cmdName = "docker"
	cmdArgs = []string{"run", "--rm"}
	validPorts, portError := checkPortInput(ports)
	if !validPorts {
		Error.logf("Ports provided as input to the command are not valid: %v\n", portError)
		os.Exit(1)
	}
	cmdArgs = processPorts(cmdArgs)
	cmdArgs = append(cmdArgs, "--name", containerName)
	if dockerNetwork != "" {
		cmdArgs = append(cmdArgs, "--network", dockerNetwork)
	}
	if getEnvVarBool("APPSODY_USER_RUN_AS_LOCAL") && runtime.GOOS != "windows" {
		current, _ := user.Current()
		cmdArgs = append(cmdArgs, "-u", fmt.Sprintf("%s:%s", current.Uid, current.Gid))
		cmdArgs = append(cmdArgs, "-e", fmt.Sprintf("APPSODY_USER=%s", current.Uid), "-e", fmt.Sprintf("APPSODY_GROUP=%s", current.Gid))
	}

	if len(volumeMaps) > 0 {
		cmdArgs = append(cmdArgs, volumeMaps...)
	}

	cmdArgs = append(cmdArgs, "-t", "--entrypoint", "/appsody/appsody-controller", platformDefinition, "--mode="+mode)
	Debug.logf("Attempting to start image %s with container name %s", platformDefinition, containerName)
	execCmd, err := execAndListen(cmdName, cmdArgs, Container)
	if dryrun {
		Info.log("Dry Run - Skipping execCmd.Wait")
	} else {
		if err == nil {
			err = execCmd.Wait()
		}
	}
	if err != nil {
		// 'signal: interrupt'
		// TODO presumably you can query the error itself
		error := fmt.Sprintf("%s", err)
		//Linux and Windows return a different error on Ctrl-C
		if error == "signal: interrupt" || error == "exit status 2" {
			Info.log("Closing down development environment, sleeping 60 seconds: ", error)

			time.Sleep(60 * time.Second)
		} else {
			Error.logf("Error waiting in 'appsody %s' %s", mode, error)

			os.Exit(1)
		}

	}

}

func processPorts(cmdArgs []string) []string {

	var exposedPortsMapping []string

	dockerExposedPorts := getExposedPorts()
	Debug.log("Exposed ports provided by the docker file", dockerExposedPorts)
	// if the container port is not in the lised of exposed ports add it to the list

	containerPort := getEnvVar("PORT")
	containerPortIsExposed := false

	Debug.log("Container port set to: ", containerPort)
	if containerPort != "" {
		for i := 0; i < len(dockerExposedPorts); i++ {

			if containerPort == dockerExposedPorts[i] {
				containerPortIsExposed = true
			}
		}
		if !containerPortIsExposed {
			dockerExposedPorts = append(dockerExposedPorts, containerPort)
		}
	}

	if publishAllPorts {
		cmdArgs = append(cmdArgs, "-P")
		// user specified to publish all EXPOSE ports to random ports with -P, so clear this list so we don't add them with -p
		dockerExposedPorts = []string{}
		if containerPort != "" && !containerPortIsExposed {
			// A PORT var was defined in the stack but not EXPOSE. It won't get published with -P, so add it as -p
			dockerExposedPorts = append(dockerExposedPorts, containerPort)
		}
	}

	Debug.log("Published ports provided as inputs: ", ports)
	for i := 0; i < len(ports); i++ { // this is the list of input -p's

		exposedPortsMapping = append(exposedPortsMapping, ports[i])

	}
	// see if there are any exposed ports (including container port) for which there are no overrides and add those to the list
	for i := 0; i < len(dockerExposedPorts); i++ {
		overrideFound := false
		for j := 0; j < len(ports); j++ {
			portMapping := strings.Split(ports[j], ":")
			if dockerExposedPorts[i] == portMapping[1] {
				overrideFound = true
			}
		}
		if !overrideFound {
			exposedPortsMapping = append(exposedPortsMapping, dockerExposedPorts[i]+":"+dockerExposedPorts[i])
		}
	}

	for k := 0; k < len(exposedPortsMapping); k++ {
		cmdArgs = append(cmdArgs, "-p", exposedPortsMapping[k])
	}
	return cmdArgs
}
func checkPortInput(publishedPorts []string) (bool, error) {
	validPorts := true
	var portError error
	validPortNumber := regexp.MustCompile("^([0-9]{1,4}|[1-5][0-9]{4}|6[0-4][0-9]{3}|65[0-4][0-9]{2}|655[0-2][0-9]|6553[0-5])$")
	for i := 0; i < len(publishedPorts); i++ {
		if !strings.Contains(publishedPorts[i], ":") {
			validPorts = false
			portError = errors.New("The port input: " + publishedPorts[i] + " is not valid as the : separator is missing.")
			break
		} else {
			// check the numbers
			portValues := strings.Split(publishedPorts[i], ":")
			fmt.Println(portValues)
			if !validPortNumber.MatchString(portValues[0]) || !validPortNumber.MatchString(portValues[1]) {
				portError = errors.New("The numeric port input: " + publishedPorts[i] + " is not valid.")
				validPorts = false
				break

			}

		}
	}
	return validPorts, portError

}
