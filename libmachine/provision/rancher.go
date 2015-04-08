package provision

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"path"
	"text/template"

	"github.com/docker/machine/drivers"
	"github.com/docker/machine/libmachine/auth"
	"github.com/docker/machine/libmachine/provision/pkgaction"
	"github.com/docker/machine/libmachine/swarm"
)

const (
	rancherTmpl = `user_docker:
  tls: true
  tls_args: [--tlsverify, --tlscacert={{ .CaCertPath }}, --tlscert={{ .ServerCertPath }}, --tlskey={{ .ServerKeyPath }},
    '-H=0.0.0.0:{{ .DockerPort }}']
  args: [docker, -d, -s, overlay, -G, docker, -H, 'unix://']
`
)

var (
	ErrUnsupportedService = errors.New("unsupported service")
)

type RancherConfig struct {
	CaCertPath     string
	ServerCertPath string
	ServerKeyPath  string
	DockerPort     int
}

func init() {
	Register("rancheros", &RegisteredProvisioner{
		New: NewRancherProvisioner,
	})
}

func NewRancherProvisioner(d drivers.Driver) Provisioner {
	return &RancherProvisioner{
		Driver: d,
	}
}

type RancherProvisioner struct {
	OsReleaseInfo *OsRelease
	Driver        drivers.Driver
	SwarmOptions  swarm.SwarmOptions
}

func (provisioner *RancherProvisioner) Service(name string, action pkgaction.ServiceAction) error {
	var (
		cmd *exec.Cmd
		err error
	)

	switch action {
	case pkgaction.Start:
		cmd, err = provisioner.SSHCommand("sudo docker -H unix:///var/run/system-docker.sock start userdocker")
	case pkgaction.Stop:
		cmd, err = provisioner.SSHCommand("sudo docker -H unix:///var/run/system-docker.sock stop userdocker")
	default:
		return ErrUnsupportedService
	}

	if err != nil {
		return err
	}

	if err = cmd.Run(); err != nil {
		return err
	}

	return nil
}

func (provisioner *RancherProvisioner) Package(name string, action pkgaction.PackageAction) error {
	return nil
}

func (provisioner *RancherProvisioner) Hostname() (string, error) {
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

func (provisioner *RancherProvisioner) SetHostname(hostname string) error {
	// TODO: this is temporary as it is in the RO filesystem
	// ideally this would be persisted in /var/lib/rancher/state/etc/hostname
	cmd, err := provisioner.SSHCommand(fmt.Sprintf(
		"sudo hostname %s && echo %q | sudo tee /etc/hostname",
		hostname,
		hostname,
	))
	if err != nil {
		return err
	}

	return cmd.Run()
}

func (provisioner *RancherProvisioner) GetDockerOptionsDir() string {
	return "/var/lib/rancher"
}

func (provisioner *RancherProvisioner) GenerateDockerOptions(dockerPort int, authOptions auth.AuthOptions) (*DockerOptions, error) {
	var buf bytes.Buffer
	cfg := &RancherConfig{
		CaCertPath:     authOptions.CaCertRemotePath,
		ServerCertPath: authOptions.ServerCertRemotePath,
		ServerKeyPath:  authOptions.ServerKeyRemotePath,
		DockerPort:     dockerPort,
	}

	t := template.Must(template.New("rancher").Parse(rancherTmpl))
	t.Execute(&buf, cfg)
	daemonCfg := buf.String()
	daemonOptsDir := path.Join(provisioner.GetDockerOptionsDir(), "conf", "rancher.yml")

	return &DockerOptions{
		EngineOptions:     daemonCfg,
		EngineOptionsPath: daemonOptsDir,
	}, nil
}

func (provisioner *RancherProvisioner) CompatibleWithHost() bool {
	return provisioner.OsReleaseInfo.Id == "rancheros"
}

func (provisioner *RancherProvisioner) SetOsReleaseInfo(info *OsRelease) {
	provisioner.OsReleaseInfo = info
}

func (provisioner *RancherProvisioner) Provision(swarmOptions swarm.SwarmOptions, authOptions auth.AuthOptions) error {
	if err := provisioner.SetHostname(provisioner.Driver.GetMachineName()); err != nil {
		return err
	}

	if err := installDockerGeneric(provisioner); err != nil {
		return err
	}

	if err := ConfigureAuth(provisioner, authOptions); err != nil {
		return err
	}

	//var (
	//	cmd    *exec.Cmd
	//	cmdErr error
	//)

	//// activate TLS config for rancher
	//cmd, cmdErr = provisioner.SSHCommand("sudo rancherctl config set -- user_docker.tls_ca_cert \"$(</home/rancher/ca.pem)\"")
	//if cmdErr != nil {
	//	return cmdErr
	//}

	//if cmdErr = cmd.Run(); cmdErr != nil {
	//	return cmdErr
	//}

	//cmd, cmdErr = provisioner.SSHCommand("sudo rancherctl config set -- user_docker.tls_server_cert \"$(</home/rancher/server.pem)\"")
	//if cmdErr != nil {
	//	return cmdErr
	//}

	//if cmdErr = cmd.Run(); cmdErr != nil {
	//	return cmdErr
	//}

	//cmd, cmdErr = provisioner.SSHCommand("sudo rancherctl config set -- user_docker.tls_server_key \"$(</home/rancher/server-key.pem)\"")
	//if cmdErr != nil {
	//	return cmdErr
	//}

	//if cmdErr = cmd.Run(); cmdErr != nil {
	//	return cmdErr
	//}

	if err := configureSwarm(provisioner, swarmOptions); err != nil {
		return err
	}

	return nil
}

func (provisioner *RancherProvisioner) SSHCommand(args ...string) (*exec.Cmd, error) {
	return drivers.GetSSHCommandFromDriver(provisioner.Driver, args...)
}

func (provisioner *RancherProvisioner) GetDriver() drivers.Driver {
	return provisioner.Driver
}

func (provisioner *RancherProvisioner) GetDockerEnginerConfigCmd(engineOptions, engineOptionsPath string) (*exec.Cmd, error) {
	return provisioner.SSHCommand(fmt.Sprintf("echo \"%s\" | sudo tee %s", engineOptions, engineOptionsPath))
}
