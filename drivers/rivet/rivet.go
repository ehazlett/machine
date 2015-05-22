package rivet

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"time"

	"github.com/codegangsta/cli"
	"github.com/docker/machine/drivers"
	"github.com/docker/machine/drivers/rivet/rvt"
	"github.com/docker/machine/log"
	"github.com/docker/machine/ssh"
	"github.com/docker/machine/state"
)

type Driver struct {
	MachineName    string
	APIEndpoint    string
	CPU            int
	Memory         int
	Storage        int
	SSHUser        string
	SSHPort        int
	CaCertPath     string
	PrivateKeyPath string
	DriverKeyPath  string
	SwarmMaster    bool
	SwarmHost      string
	SwarmDiscovery string
	storePath      string
}

const (
	defaultTimeout = 1 * time.Second
)

func init() {
	drivers.Register("rivet", &drivers.RegisteredDriver{
		New:            NewDriver,
		GetCreateFlags: GetCreateFlags,
	})
}

// GetCreateFlags registers the flags this driver adds to
// "docker hosts create"
func GetCreateFlags() []cli.Flag {
	return []cli.Flag{
		cli.StringFlag{
			Name:   "rivet-address",
			Usage:  "Address of rivet API endpoint",
			Value:  "",
			EnvVar: "RIVET_ADDRESS",
		},
		cli.IntFlag{
			Name:   "rivet-cpu",
			Usage:  "CPU for rivet instance",
			Value:  1,
			EnvVar: "RIVET_CPU",
		},
		cli.IntFlag{
			Name:   "rivet-memory",
			Usage:  "Memory for rivet instance (in MB)",
			Value:  1024,
			EnvVar: "RIVET_MEMORY",
		},
		cli.IntFlag{
			Name:   "rivet-storage",
			Usage:  "Storage for rivet instance (in GB)",
			Value:  10,
			EnvVar: "RIVET_STORAGE",
		},
		cli.StringFlag{
			Name:   "rivet-ssh-user",
			Usage:  "SSH user for rivet instance",
			Value:  "root",
			EnvVar: "RIVET_SSH_USER",
		},
	}
}

func NewDriver(machineName string, storePath string, caCert string, privateKey string) (drivers.Driver, error) {
	return &Driver{
		MachineName:    machineName,
		storePath:      storePath,
		CaCertPath:     caCert,
		PrivateKeyPath: privateKey,
	}, nil
}

func (d *Driver) DriverName() string {
	return "rivet"
}

func (d *Driver) AuthorizePort(ports []*drivers.Port) error {
	return nil
}

func (d *Driver) DeauthorizePort(ports []*drivers.Port) error {
	return nil
}

func (d *Driver) GetMachineName() string {
	return d.MachineName
}

func (d *Driver) GetSSHHostname() (string, error) {
	return d.GetIP()
}

func (d *Driver) GetSSHKeyPath() string {
	return filepath.Join(d.storePath, "id_rsa")
}

func (d *Driver) GetSSHPort() (int, error) {
	if d.SSHPort == 0 {
		d.SSHPort = 22
	}

	return d.SSHPort, nil
}

func (d *Driver) GetSSHUsername() string {
	return d.SSHUser
}

func (d *Driver) SetConfigFromFlags(flags drivers.DriverOptions) error {
	d.APIEndpoint = flags.String("rivet-address")
	d.CPU = flags.Int("rivet-cpu")
	d.Memory = flags.Int("rivet-memory")
	d.Storage = flags.Int("rivet-storage")
	d.SSHUser = flags.String("rivet-ssh-user")

	if d.APIEndpoint == "" {
		return fmt.Errorf("rivet driver requires the --rivet-address option")
	}

	return nil
}

func (d *Driver) PreCreateCheck() error {
	return nil
}

func (d *Driver) getAPI() (*rvt.RivetAPI, error) {
	return rvt.NewRivetAPI(d.APIEndpoint)
}

func (d *Driver) Create() error {
	log.Infof("Creating Rivet Instance...")

	key, err := d.createSSHKey()
	if err != nil {
		return err
	}

	r, err := d.getAPI()
	if err != nil {
		return err
	}

	resp, err := r.Create(d.MachineName, key, d.CPU, d.Memory, d.Storage)
	if err != nil {
		return err
	}

	if resp.StatusCode != 200 {
		log.Fatalf("error creating machine: %s", resp.Response)
	}

	log.Debug(resp.Response)
	return nil
}

func (d *Driver) GetURL() (string, error) {
	ip, err := d.GetIP()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("tcp://%s:2376", ip), nil
}

func (d *Driver) GetIP() (string, error) {
	r, err := d.getAPI()
	if err != nil {
		return "", err
	}

	resp, err := r.GetIP(d.MachineName)
	if err != nil {
		log.Error(err)
		return "", err
	}
	if resp.StatusCode != 200 {
		return "", fmt.Errorf(resp.Response)
	}

	log.Debugf("ip: %s", resp.Response)
	return resp.Response, nil
}

func (d *Driver) GetState() (state.State, error) {
	r, err := d.getAPI()
	if err != nil {
		return state.Error, err
	}

	resp, err := r.GetState(d.MachineName)
	if err != nil {
		return state.Error, err
	}

	if resp.StatusCode != 200 {
		return state.Error, fmt.Errorf(resp.Response)
	}

	var st state.State
	switch resp.Response {
	case "running":
		st = state.Running
	case "stopped":
		st = state.Stopped
	case "pending":
		st = state.Starting
	default:
		st = state.None
	}
	return st, nil
}

func (d *Driver) Start() error {
	r, err := d.getAPI()
	if err != nil {
		return err
	}

	resp, err := r.Start(d.MachineName)
	if err != nil {
		log.Error(err)
		return err
	}

	if resp.StatusCode != 200 {
		return fmt.Errorf(resp.Response)
	}

	log.Debug(resp.Response)
	return nil
}

func (d *Driver) Stop() error {
	r, err := d.getAPI()
	if err != nil {
		return err
	}

	resp, err := r.Stop(d.MachineName)
	if err != nil {
		log.Error(err)
		return err
	}

	if resp.StatusCode != 200 {
		return fmt.Errorf(resp.Response)
	}

	log.Debug(resp.Response)
	return nil
}

func (d *Driver) Remove() error {
	r, err := d.getAPI()
	if err != nil {
		return err
	}

	resp, err := r.Remove(d.MachineName)
	if err != nil {
		log.Error(err)
		return err
	}

	if resp.StatusCode != 200 {
		return fmt.Errorf(resp.Response)
	}

	log.Debug(resp.Response)
	return nil
}

func (d *Driver) Restart() error {
	r, err := d.getAPI()
	if err != nil {
		return err
	}

	resp, err := r.Restart(d.MachineName)
	if err != nil {
		log.Error(err)
		return err
	}

	if resp.StatusCode != 200 {
		return fmt.Errorf(resp.Response)
	}

	log.Debug(resp.Response)
	return nil
}

func (d *Driver) Kill() error {
	r, err := d.getAPI()
	if err != nil {
		return err
	}

	resp, err := r.Kill(d.MachineName)
	if err != nil {
		log.Error(err)
		return err
	}

	if resp.StatusCode != 200 {
		return fmt.Errorf(resp.Response)
	}

	log.Debug(resp.Response)
	return nil
}

func (d *Driver) sshKeyPath() string {
	return filepath.Join(d.storePath, "id_rsa")
}

func (d *Driver) publicSSHKeyPath() string {
	return d.sshKeyPath() + ".pub"
}

func (d *Driver) createSSHKey() ([]byte, error) {
	if err := ssh.GenerateSSHKey(d.sshKeyPath()); err != nil {
		return nil, err
	}

	publicKey, err := ioutil.ReadFile(d.publicSSHKeyPath())
	if err != nil {
		return nil, err
	}

	return publicKey, nil
}
