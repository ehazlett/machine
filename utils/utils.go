package utils

import (
	"io"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

func GetHomeDir() string {
	if runtime.GOOS == "windows" {
		return os.Getenv("USERPROFILE")
	}
	return os.Getenv("HOME")
}

func GetBaseDir() string {
	baseDir := os.Getenv("MACHINE_DIR")
	if baseDir == "" {
		baseDir = GetHomeDir()
	}
	return baseDir
}

func GetDockerDir() string {
	return filepath.Join(GetBaseDir(), ".docker")
}

func GetMachineDir() string {
	return filepath.Join(GetDockerDir(), "machines")
}

func GetMachineClientCertDir() string {
	return filepath.Join(GetMachineDir(), ".client")
}

func GetUsername() string {
	u := "unknown"
	osUser := ""

	switch runtime.GOOS {
	case "darwin", "linux":
		osUser = os.Getenv("USER")
	case "windows":
		osUser = os.Getenv("USERNAME")
	}

	if osUser != "" {
		u = osUser
	}

	return u
}

func CopyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}

	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}

	if _, err = io.Copy(out, in); err != nil {
		return err
	}

	return nil
}

// WaitForDocker will retry until either a successful connection or maximum retries is reached
func WaitForDocker(url string, maxRetries int) bool {
	counter := 0
	for {
		conn, err := net.DialTimeout("tcp", url, time.Duration(1)*time.Second)
		if err != nil {
			counter++
			if counter == maxRetries {
				return false

			}
			time.Sleep(1 * time.Second)
			continue

		}
		defer conn.Close()
		break
	}

	return true
}
