package provision

import (
	"bytes"
	"fmt"
	"os/exec"
	"path"

	"github.com/docker/machine/drivers"
	"github.com/docker/machine/libmachine/auth"
	"github.com/docker/machine/libmachine/provision/pkgaction"
	"github.com/docker/machine/libmachine/swarm"
	"github.com/docker/machine/utils"
)

func init() {
	Register("boot2docker", &RegisteredProvisioner{
		New: NewBoot2DockerProvisioner,
	})
}

func NewBoot2DockerProvisioner(d drivers.Driver) Provisioner {
	return &Boot2DockerProvisioner{
		Driver: d,
	}
}

type Boot2DockerProvisioner struct {
	OsReleaseInfo *OsRelease
	Driver        drivers.Driver
	SwarmConfig   swarm.SwarmOptions
}

func (provisioner *Boot2DockerProvisioner) Service(name string, action pkgaction.ServiceAction) error {
	var (
		cmd *exec.Cmd
		err error
	)
	if name == "docker" && action == pkgaction.Stop {
		cmd, err = provisioner.SSHCommand("if [ -e /var/run/docker.pid  ] && [ -d /proc/$(cat /var/run/docker.pid)  ]; then sudo /etc/init.d/docker stop ; exit 0; fi")
	} else {
		cmd, err = provisioner.SSHCommand(fmt.Sprintf("sudo /etc/init.d/%s %s", name, action.String()))
		if err != nil {
			return err
		}
	}
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}

func (provisioner *Boot2DockerProvisioner) Package(name string, action pkgaction.PackageAction) error {
	return nil
}

func (provisioner *Boot2DockerProvisioner) Hostname() (string, error) {
	cmd, err := provisioner.SSHCommand(fmt.Sprintf("hostname"))
	if err != nil {
		return "", err
	}

	var so bytes.Buffer
	cmd.Stdout = &so

	if err := cmd.Run(); err != nil {
		return "", err
	}

	return so.String(), nil
}

func (provisioner *Boot2DockerProvisioner) SetHostname(hostname string) error {
	cmd, err := provisioner.SSHCommand(fmt.Sprintf(
		"sudo hostname %s && echo %q | sudo tee /var/lib/boot2docker/etc/hostname",
		hostname,
		hostname,
	))
	utils.DumpVal(cmd)
	if err != nil {
		return err
	}

	return cmd.Run()
}

func (provisioner *Boot2DockerProvisioner) GetDockerConfigDir() string {
	return "/var/lib/boot2docker"
}

func (provisioner *Boot2DockerProvisioner) GenerateDockerConfig(dockerPort int, authConfig auth.AuthOptions) (*DockerConfig, error) {
	defaultDaemonOpts := getDefaultDaemonOpts(provisioner.Driver.DriverName(), authConfig)
	daemonOpts := fmt.Sprintf("-H tcp://0.0.0.0:%d", dockerPort)
	daemonOptsCfg := path.Join(provisioner.GetDockerConfigDir(), "profile")
	opts := fmt.Sprintf("%s %s", defaultDaemonOpts, daemonOpts)
	daemonCfg := fmt.Sprintf(`EXTRA_ARGS='%s'
CACERT=%s
SERVERCERT=%s
SERVERKEY=%s
DOCKER_TLS=no`, opts, authConfig.CaCertPath, authConfig.ServerKeyPath, authConfig.ServerCertPath)
	return &DockerConfig{
		EngineConfig:     daemonCfg,
		EngineConfigPath: daemonOptsCfg,
	}, nil
}

func (provisioner *Boot2DockerProvisioner) CompatibleWithHost() bool {
	return provisioner.OsReleaseInfo.Id == "boot2docker"
}

func (provisioner *Boot2DockerProvisioner) SetOsReleaseInfo(info *OsRelease) {
	provisioner.OsReleaseInfo = info
}

func (provisioner *Boot2DockerProvisioner) Provision(swarmConfig swarm.SwarmOptions, authConfig auth.AuthOptions) error {
	fmt.Println("before set hostname")
	if err := provisioner.SetHostname(provisioner.Driver.GetMachineName()); err != nil {
		return err
	}
	fmt.Println("after set hostname")

	if err := installDockerGeneric(provisioner); err != nil {
		return err
	}

	if err := ConfigureAuth(provisioner, authConfig); err != nil {
		return err
	}

	if err := configureSwarm(provisioner, swarmConfig); err != nil {
		return err
	}

	return nil
}

func (provisioner *Boot2DockerProvisioner) SSHCommand(args ...string) (*exec.Cmd, error) {
	return drivers.GetSSHCommandFromDriver(provisioner.Driver, args...)
}

func (provisioner *Boot2DockerProvisioner) GetDriver() drivers.Driver {
	return provisioner.Driver
}
