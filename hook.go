package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/opencontainers/runc/libcontainer/configs"
	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/sirupsen/logrus"
)

var (
	// version is the version string of the hook. Set at build time.
	version string
	log     = logrus.New()
)

const (
	//Raksh encrypted data
	rakshProperties = "raksh.properties"
	//Memory mapped directory inside the Kata VM
	vmMemDir = "/run/svm"
	//Shared memory mapped directory within the Kata VM and the containers
	containerSharedMemDir = "/run/kata-containers/sandbox/shm"
)

func main() {

	log.Out = os.Stdout

	dname, err := ioutil.TempDir("", "hooklog")
	fname := filepath.Join(dname, "hook.log")
	file, err := os.OpenFile(fname, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err == nil {
		log.Infof("Log file: %s", fname)
		log.Out = file
	} else {
		log.Info("Failed to log to file, using default stderr")
	}
	log.Info("Started OCI hook version %s", version)

	start := flag.Bool("s", true, "Start the hook")
	printVersion := flag.Bool("version", false, "Print the hook's version")
	flag.Parse()

	if *printVersion {
		fmt.Println(version)
		os.Exit(0)
	}

	if *start {
		log.Info("Starting actual hook")
		if err := modifyMount(); err != nil {
			log.Fatal(err)
		}
	}
}

// Modify the Raksh secrets mount-point
func modifyMount() error {
	//Hook receives container State in Stdin
	//https://github.com/opencontainers/runtime-spec/blob/master/config.md#posix-platform-hooks
	//https://github.com/opencontainers/runtime-spec/blob/master/runtime.md#state
	var s spec.State
	reader := bufio.NewReader(os.Stdin)
	decoder := json.NewDecoder(reader)
	err := decoder.Decode(&s)
	if err != nil {
		return err
	}

	//log spec to file
	log.Infof("spec.State is %v", s)

	//Take out the config.json from the bundle and edit the mount points
	configJsonPath := filepath.Join(s.Bundle, "config.json")

	log.Infof("Config.json location: %s", configJsonPath)
	//Read the JSON
	var config configs.Config
	jsonData, err := ioutil.ReadFile(configJsonPath)
	if err != nil {
		log.Errorf("unable to read config.json %s", err)
		return err
	}
	err = json.Unmarshal(jsonData, &config)
	if err != nil {
		log.Errorf("unable to unmarshal config.json %s", err)
		return err
	}
	for _, m := range config.Mounts {
		log.Infof("src: %s  ==  dest: %s", m.Source, m.Destination)
		//Check if dest is raksh
		if strings.Contains(m.Destination, "raksh") == true {
			//Read the contents and log
			//The src is a directory
			rakshSrcFile := filepath.Join(m.Source, rakshProperties)
			rakshSrcContent, _ := ioutil.ReadFile(rakshSrcFile)
			log.Infof("Raksh src data %s", string(rakshSrcContent))

			//This will be empty since the src has not yet been mounted in the prestart phase
			//rakshDestContent, _ := ioutil.ReadFile(m.Destination)
			//log.Infof("Raksh dst data %s", string(rakshDestContent))

			//Copy the data to VM's memory. This is not share with the containers
			err = os.MkdirAll(vmMemDir, os.ModeDir)
			if err != nil {
				log.Infof("Error creating vmMemDir", err)
				return err
			}
			vmMemDirFile := filepath.Join(vmMemDir, rakshProperties)

			err = ioutil.WriteFile(vmMemDirFile, rakshSrcContent, 0644)
			if err != nil {
				log.Infof("Error writing the data to vmMemDirFile", err)
				return err
			}
			//Shared memory between VM and containers
			containerSharedMemDirFile := filepath.Join(containerSharedMemDir, rakshProperties)
			err = ioutil.WriteFile(containerSharedMemDirFile, rakshSrcContent, 0644)
			if err != nil {
				log.Infof("Error writing the data to containerSharedMemDirFile", err)
				return err
			}
			break
		}
	}

	log.Debugf("Config struct %s\n", string(jsonData))
	return nil
}
