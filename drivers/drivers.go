package drivers

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"io/ioutil"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"text/template"

	log "github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"
	"github.com/docker/machine/state"
	"github.com/docker/machine/utils"
)

const (
	cloudInitTemplate = `#cloud-config
apt_update: true
apt_sources:
  - source: "deb https://get.docker.com/ubuntu docker main"
    filename: docker.list
    keyserver: keyserver.ubuntu.com
    keyid: A88D21E9

package_update: true

packages:
  - lxc-docker

write_files:
  - encoding: base64
    content: {{ .DockerOptsBase64 }}
    path: {{ .DockerConfig.EngineConfigPath }}
    permissions: 0644
  - encoding: base64
    content: {{ .CaCertBase64 }}
    path: {{ .MachineOpts.CaCertPath }}
    permissions: 0644
  - encoding: base64
    content: {{ .ServerCertBase64 }}
    path: {{ .MachineOpts.ServerCertPath }}
    permissions: 0644
  - encoding: base64
    content: {{ .ServerKeyBase64 }}
    path: {{ .MachineOpts.ServerKeyPath }}
    permissions: 0644

runcmd:
  - [ stop, docker ]
  - [ start, docker ]

final_message: "Docker Machine provisioning complete"
`
)

type (
	CloudInitOptions struct {
		MachineOpts      *MachineOptions
		DockerConfig     *DockerConfig
		DockerOptsBase64 string
		CaCertBase64     string
		ServerCertBase64 string
		ServerKeyBase64  string
	}

	DockerConfig struct {
		EngineConfig     string
		EngineConfigPath string
	}
)

// Driver defines how a host is created and controlled. Different types of
// driver represent different ways hosts can be created (e.g. different
// hypervisors, different cloud providers)
type Driver interface {
	// DriverName returns the name of the driver as it is registered
	DriverName() string

	// SetConfigFromFlags configures the driver with the object that was returned
	// by RegisterCreateFlags
	SetConfigFromFlags(flags DriverOptions) error

	// GetURL returns a Docker compatible host URL for connecting to this host
	// e.g. tcp://1.2.3.4:2376
	GetURL() (string, error)

	// GetIP returns an IP or hostname that this host is available at
	// e.g. 1.2.3.4 or docker-host-d60b70a14d3a.cloudapp.net
	GetIP() (string, error)

	// GetState returns the state that the host is in (running, stopped, etc)
	GetState() (state.State, error)

	// PreCreate allows for pre-create operations to make sure a driver is ready for creation
	PreCreateCheck() error

	// Create a host using the driver's config
	Create() error

	// Remove a host
	Remove() error

	// Start a host
	Start() error

	// Stop a host gracefully
	Stop() error

	// Restart a host. This may just call Stop(); Start() if the provider does not
	// have any special restart behaviour.
	Restart() error

	// Kill stops a host forcefully
	Kill() error

	// RestartDocker restarts a Docker daemon on the machine
	StartDocker() error

	// RestartDocker restarts a Docker daemon on the machine
	StopDocker() error

	// Upgrade the version of Docker on the host to the latest version
	Upgrade() error

	// GetDockerConfigDir returns the config directory for storing daemon configs
	GetDockerConfigDir() string

	// GetMachineName returns the name of the machine
	GetMachineName() string

	// GetCACertPath returns the CA cert path
	GetCACertPath() string

	// GetCAKeyPath returns the CA key path
	GetCAKeyPath() string

	// GetSSHCommand returns a command for SSH pointing at the correct user, host
	// and keys for the host with args appended. If no args are passed, it will
	// initiate an interactive SSH session as if SSH were passed no args.
	GetSSHCommand(args ...string) (*exec.Cmd, error)
}

// RegisteredDriver is used to register a driver with the Register function.
// It has two attributes:
// - New: a function that returns a new driver given a path to store host
//   configuration in
// - RegisterCreateFlags: a function that takes the FlagSet for
//   "docker hosts create" and returns an object to pass to SetConfigFromFlags
type RegisteredDriver struct {
	New            func(machineName string, storePath string, caCert string, privateKey string) (Driver, error)
	GetCreateFlags func() []cli.Flag
}

var ErrHostIsNotRunning = errors.New("host is not running")

var (
	drivers map[string]*RegisteredDriver
)

func init() {
	drivers = make(map[string]*RegisteredDriver)
}

// Register a driver
func Register(name string, registeredDriver *RegisteredDriver) error {
	if _, exists := drivers[name]; exists {
		return fmt.Errorf("Name already registered %s", name)
	}

	drivers[name] = registeredDriver
	return nil
}

// NewDriver creates a new driver of type "name"
func NewDriver(name string, machineName string, storePath string, caCert string, privateKey string) (Driver, error) {
	driver, exists := drivers[name]
	if !exists {
		return nil, fmt.Errorf("hosts: Unknown driver %q", name)
	}
	return driver.New(machineName, storePath, caCert, privateKey)
}

// GetCreateFlags runs GetCreateFlags for all of the drivers and
// returns their return values indexed by the driver name
func GetCreateFlags() []cli.Flag {
	flags := []cli.Flag{}

	for driverName := range drivers {
		driver := drivers[driverName]
		for _, f := range driver.GetCreateFlags() {
			flags = append(flags, f)
		}
	}

	sort.Sort(ByFlagName(flags))

	return flags
}

// GetDriverNames returns a slice of all registered driver names
func GetDriverNames() []string {
	names := make([]string, 0, len(drivers))
	for k := range drivers {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}

type DriverOptions interface {
	String(key string) string
	Int(key string) int
	Bool(key string) bool
}

func GenerateCloudInit(d Driver, machineOpts *MachineOptions) (string, error) {
	if d.DriverName() == "none" {
		return "", nil
	}

	machineCaCertPath := path.Join(d.GetDockerConfigDir(), "ca.pem")
	machineServerCertPath := path.Join(d.GetDockerConfigDir(), "server.pem")
	machineServerKeyPath := path.Join(d.GetDockerConfigDir(), "server-key.pem")

	if machineOpts == nil {
		machineOpts = &MachineOptions{
			Host:           "tcp://0.0.0.0:2376",
			Labels:         []string{},
			CaCertPath:     machineCaCertPath,
			ServerCertPath: machineServerCertPath,
			ServerKeyPath:  machineServerKeyPath,
		}
	}

	caCert, err := ioutil.ReadFile(d.GetCACertPath())
	if err != nil {
		return "", err
	}

	serverCert, serverKey, err := GenerateMachineCerts(d)
	if err != nil {
		return "", err
	}

	encodedCaCert := base64.StdEncoding.EncodeToString(caCert)
	encodedServerCert := base64.StdEncoding.EncodeToString(serverCert)
	encodedServerKey := base64.StdEncoding.EncodeToString(serverKey)

	dockerConfig := GenerateDockerConfig(d, machineOpts)

	buf := bytes.NewBufferString(dockerConfig.EngineConfig)
	encodedDockerOpts := base64.StdEncoding.EncodeToString(buf.Bytes())

	cloudInitOpts := &CloudInitOptions{
		MachineOpts:      machineOpts,
		DockerConfig:     dockerConfig,
		DockerOptsBase64: encodedDockerOpts,
		CaCertBase64:     encodedCaCert,
		ServerCertBase64: encodedServerCert,
		ServerKeyBase64:  encodedServerKey,
	}

	var tmpl bytes.Buffer
	t := template.Must(template.New("machine-cloud-init").Parse(cloudInitTemplate))
	if err := t.Execute(&tmpl, cloudInitOpts); err != nil {
		return "", err
	}

	log.Debug("cloud config: ")
	log.Debug(tmpl.String())

	return tmpl.String(), nil
}

func GenerateCloudInitBase64(d Driver, machineOpts *MachineOptions) (string, error) {
	config, err := GenerateCloudInit(d, machineOpts)
	if err != nil {
		return "", err
	}

	buf := bytes.NewBufferString(config)
	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

func GenerateDockerConfig(d Driver, machineOpts *MachineOptions) *DockerConfig {
	var (
		daemonOpts    string
		daemonOptsCfg string
		daemonCfg     string
	)

	defaultDaemonOpts := fmt.Sprintf(`--tlsverify --tlscacert=%s --tlskey=%s --tlscert=%s`, machineOpts.CaCertPath, machineOpts.ServerKeyPath, machineOpts.ServerCertPath)
	switch d.DriverName() {
	case "virtualbox", "vmwarefusion", "vmwarevsphere":
		daemonOpts = fmt.Sprintf("-H %s", machineOpts.Host)
		daemonOptsCfg = filepath.Join(d.GetDockerConfigDir(), "profile")
		opts := fmt.Sprintf("%s %s", defaultDaemonOpts, daemonOpts)
		daemonCfg = fmt.Sprintf(`EXTRA_ARGS='%s'
CACERT=%s
SERVERCERT=%s
SERVERKEY=%s
DOCKER_TLS=no`, opts, machineOpts.CaCertPath, machineOpts.ServerKeyPath, machineOpts.ServerCertPath)
	default:
		daemonOpts = fmt.Sprintf("--host=unix:///var/run/docker.sock --host=%s", machineOpts.Host)
		daemonOptsCfg = "/etc/default/docker"
		opts := fmt.Sprintf("%s %s", defaultDaemonOpts, daemonOpts)
		daemonCfg = fmt.Sprintf("export DOCKER_OPTS='%s'", opts)
	}

	return &DockerConfig{
		EngineConfig:     daemonCfg,
		EngineConfigPath: daemonOptsCfg,
	}
}

func GenerateMachineCerts(d Driver) (serverCert []byte, serverKey []byte, err error) {
	if d.DriverName() == "none" {
		return nil, nil, nil
	}

	org := d.GetMachineName()
	bits := 2048

	log.Debugf("generating server cert for %s", d.GetMachineName())

	certData, keyData, err := utils.GenerateCert([]string{"*"}, d.GetCACertPath(), d.GetCAKeyPath(), org, bits)
	if err != nil {
		return nil, nil, fmt.Errorf("error generating server cert: %s", err)
	}

	return certData, keyData, err
}
