package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"syscall"
	"time"

	"k8s.io/klog"
)

const (
	secretsFilesPath = "/centrify/secrets"
)

func main() {
	var entrypointCmd []string
	if len(os.Args) == 1 {
		// a 'command' attribute must be set on images in pod manifest. If not we cannot start the expected process
		klog.Errorf("no command is explicityly provided, %s can't determine image entrypoint", os.Args[0])
		os.Exit(1)
	} else {
		entrypointCmd = os.Args[1:]
	}

	// Wait for secret files to be created in case of using sidecar method
	counter := 30
	for i := 1; i <= counter; i++ {
		empty, err := isDirEmpty(secretsFilesPath)
		if err != nil {
			klog.Errorf("Secret file path %s doesn't exist.", secretsFilesPath)
			os.Exit(1)
		}
		if empty {
			klog.Infof("Waiting for secret file %d...", i)
			time.Sleep(1 * time.Second)
		} else {
			klog.Infof("Secret file created")
			break
		}
	}

	binary, err := exec.LookPath(entrypointCmd[0])
	if err != nil {
		klog.Fatalln(err.Error())
	}

	secretsFiles, err := ioutil.ReadDir(secretsFilesPath)
	if err != nil {
		klog.Fatalln(err.Error())
	}

	// Get currently defined env vars
	env := os.Environ()

	for _, f := range secretsFiles {
		filePath := path.Join(secretsFilesPath, f.Name())
		klog.Infof("Secrets file=%s", filePath)
		content, err := ioutil.ReadFile(filePath)
		if err != nil {
			klog.Fatalln(err)
		}
		newenv := fmt.Sprintf("%s=%s", f.Name(), string(content))

		// Add to env vars. We do not check for collisions: make sure to not have same keys in secrets files (and do not use existing env keys either)
		env = append(env, newenv)
	}

	// Replace current process with original one, providing env vars (including new ones from fetched secrets)
	klog.Infof("New envs: %v\n", env)
	klog.Infof("Starting original program: %v ...\n", entrypointCmd)
	err = syscall.Exec(binary, entrypointCmd, env)
	if err != nil {
		log.Panicln("failed to exec process", entrypointCmd, err.Error())
	}
}

func isDirEmpty(name string) (bool, error) {
	f, err := os.Open(name)
	if err != nil {
		return false, err
	}
	defer f.Close()

	// read in ONLY one file
	_, err = f.Readdir(1)

	// and if the file is EOF... well, the dir is empty.
	if err == io.EOF {
		return true, nil
	}
	return false, err
}

func mainold() {
	var entrypointCmd []string
	if len(os.Args) == 1 {
		klog.Errorf("no command is given, %s can't determine the entrypoint (command)", os.Args[0])
		os.Exit(1)
	} else {
		entrypointCmd = os.Args[1:]
	}

	binary, err := exec.LookPath(entrypointCmd[0])
	if err != nil {
		klog.Errorf("binary not found %v", entrypointCmd[0])
		os.Exit(1)
	}

	// Get currently defined env vars
	env := os.Environ()

	var injectedEnvs []string
	injectedEnvs = append(injectedEnvs, "WORDPRESS_DB_PASSWORD=kjaklsdjfklajfl")

	env = append(env, injectedEnvs...)

	klog.Infoln("spawning process:", entrypointCmd)
	cmd := exec.Command(binary, entrypointCmd[1:]...)
	cmd.Env = append(os.Environ(), injectedEnvs...)
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	os.Setenv("WORDPRESS_DB_PASSWORD", "kjflsjdlfjlks")

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs)

	err = cmd.Start()
	if err != nil {
		klog.Errorf("failed to start process %v with error %v", entrypointCmd, err.Error())
		os.Exit(1)
	}

	go func() {
		for sig := range sigs {
			// We don't want to signal a non-running process.
			if cmd.ProcessState != nil && cmd.ProcessState.Exited() {
				break
			}

			err := cmd.Process.Signal(sig)
			if err != nil {
				klog.Infof("failed to signal process with %s: %v", sig, err)
			} else {
				klog.Infof("received signal: %s", sig)
			}
		}
	}()

	err = cmd.Wait()

	close(sigs)

	if _, ok := err.(*exec.ExitError); ok {
		os.Exit(cmd.ProcessState.ExitCode())
	} else if err != nil {
		klog.Fatalln("failed to exec process", entrypointCmd, err.Error())
		os.Exit(-1)
	} else {
		os.Exit(cmd.ProcessState.ExitCode())
	}
}
