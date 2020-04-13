package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
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
	rakshSharedMemDir     = "/run/kata-containers/sandbox/shm/raksh"
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
		if err := startRakshHook(); err != nil {
			log.Fatal(err)
		}
	}
}

// Modify the Raksh secrets mount-point
func startRakshHook() error {
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
	log.Debugf("spec.State is %v", s)

	bundlePath := s.Bundle
	containerPid := s.Pid

	//Get source mount path for Raksh secrets
	rakshSrcMountPath, err := getMountSrcFromConfigJson(bundlePath, "raksh")
	if (rakshSrcMountPath == "") || (err != nil) {
		log.Errorf("unable to get source mount path %s", err)
		return err
	}

	//Read the Raksh secrets
	rakshSecretData, err := readRakshSecrets(rakshSrcMountPath)
	if err != nil {
		log.Errorf("unable to read Raksh secret data %s", err)
		return err
	}

	//Decrypt the Raksh secrets
	rakshDecryptedData, err := decryptRakshSecrets(rakshSecretData)
	if err != nil {
		log.Errorf("unable to decrypt Raksh secret data %s", err)
		return err
	}

	err = writeDecryptedRakshDataToSharedDir(rakshDecryptedData, rakshSharedMemDir)
	if err != nil {
		log.Infof("Error writing the decrypted Raksh secret data to containerSharedMemDirFile", err)
		return err
	}

	err = modifyRakshBindMount(containerPid, bundlePath)
	if err != nil {
		log.Infof("Error modifying the Raksh mount point", err)
		return err
	}

	return nil
}

//Get source path of bind mount
func getMountSrcFromConfigJson(configJsonDir string, destMountPath string) (string, error) {

	var srcMountPath string
	//Take out the config.json from the bundle and edit the mount points
	configJsonPath := filepath.Join(configJsonDir, "config.json")

	log.Infof("Config.json location: %s", configJsonPath)
	//Read the JSON
	var config configs.Config
	jsonData, err := ioutil.ReadFile(configJsonPath)
	if err != nil {
		log.Errorf("unable to read config.json %s", err)
		return "", err
	}
	err = json.Unmarshal(jsonData, &config)
	if err != nil {
		log.Errorf("unable to unmarshal config.json %s", err)
		return "", err
	}
	for _, m := range config.Mounts {
		log.Infof("src: %s  ==  dest: %s", m.Source, m.Destination)
		//Check if dest is raksh
		if strings.Contains(m.Destination, destMountPath) == true {
			//Read the contents and log
			//The src is a directory
			srcMountPath = m.Source
			break
		}
	}

	log.Infof("mount src from config.json: %s", srcMountPath)

	return srcMountPath, nil

}

//Read the raksh secrets
func readRakshSecrets(srcPath string) ([]byte, error) {

	log.Infof("Raksh secret data path %s", srcPath)
	srcFile := filepath.Join(srcPath, rakshProperties)
	secretData, err := ioutil.ReadFile(srcFile)
	if err != nil {
		log.Errorf("Unable to read raksh secrets %s", err)
		return nil, err
	}

	log.Infof("Raksh secret data %s", string(secretData))
	return secretData, nil
}

//Decrypt the Raksh secrets
func decryptRakshSecrets(secretData []byte) ([]byte, error) {

	log.Infof("Decrypt Raksh secrets")
	//Decrypt the secret data - local/remote attestation etc

	return secretData, nil
}

//Copy the Raksh secret in VM memory for use with container
func writeDecryptedRakshDataToSharedDir(decryptedData []byte, destPath string) error {

	log.Infof("Write decrypted Raksh secrets to VM memory")

	err := os.MkdirAll(destPath, 0755)
	if err != nil {
		log.Infof("Error creating destPath", err)
		return err
	}

	containerSharedMemDirFile := filepath.Join(destPath, rakshProperties)
	err = ioutil.WriteFile(containerSharedMemDirFile, decryptedData, 0644)
	if err != nil {
		log.Infof("Error writing the data to containerSharedMemDirFile", err)
		return err
	}
	return nil
}

//Modify Raksh Bind mount
func modifyRakshBindMount(pid int, bundlePath string) error {

	log.Infof("modifying bind mount for process %d", pid)

	// Enter_namespaces_of_process(containerPid)
	// - mnt (/proc/containerPid/ns/mnt)
	// - pid (/proc/containerPid/ns/pid)
	// un mount /etc/raksh
	// mount /etc/raksh in tmpfs
	// Copy decrypted data from rakshSharedMemDir to /etc/raksh

	// secret is mounted in the following path /run/kata-containers/shared/containers/<container_id>/rootfs/etc/raksh
	mntDest := filepath.Join(bundlePath, "/rootfs/etc/raksh")
	args := []string{"-t", strconv.Itoa(pid), "-m", "-p", "umount", mntDest}
	cmd := exec.Command("nsenter", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Infof("Error in executing umount ", err)
		log.Infof("out ", string(out))
		return err
	}

	args = []string{"-t", strconv.Itoa(pid), "-m", "-p", "mount"}
	cmd = exec.Command("nsenter", args...)
	out, err = cmd.CombinedOutput()
	if err != nil {
		log.Infof("Error in executing mount ", err)
		log.Infof("out ", string(out))
		return err
	}

	log.Debugf("Existing mount list inside the container : ", string(out))

	//Copy secrets from rakshSharedMemDir to /etc/raksh
	srcPath := filepath.Join(rakshSharedMemDir, rakshProperties)
	destPath := filepath.Join(bundlePath, "/rootfs/etc/raksh")

	args = []string{"-m", "-p", "-t", strconv.Itoa(pid), "mount", "-t", "tmpfs", "tmpfs", destPath}
	cmd = exec.Command("nsenter", args...)
	out, err = cmd.CombinedOutput()
	if err != nil {
		log.Infof("Error in executing tmpfs mount ", err)
		log.Infof("out ", string(out))
		return err
	}

	args = []string{"-m", "-p", "-t", strconv.Itoa(pid), "cp", "-a", srcPath, destPath}
	cmd = exec.Command("nsenter", args...)
	out, err = cmd.CombinedOutput()
	if err != nil {
		log.Infof("Error in executing copy command ", err)
		log.Infof("out ", string(out))
		return err
	}
	log.Infof("ls out ", string(out))

	//Delete the secrets from rakshSharedMemDir
	args = []string{"-m", "-p", "-t", strconv.Itoa(pid), "rm", "-rf", rakshSharedMemDir}
	cmd = exec.Command("nsenter", args...)
	out, err = cmd.CombinedOutput()
	if err != nil {
		log.Infof("Error in deleting directory ", err)
		log.Infof("out ", string(out))
		return err
	}
	log.Infof("ls out ", string(out))

	log.Infof("Modifying bind mount complete")
	return nil

}
