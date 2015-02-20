package cluster

import (
	"os/exec"
	"path/filepath"
	"sync"

	log "github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"
	"github.com/docker/machine"
	"github.com/docker/machine/drivers"
	"github.com/docker/machine/state"
)

const (
	dockerConfigDir = "/etc/docker"
)

type Driver struct {
	MachineName    string
	CaCertPath     string
	PrivateKeyPath string
	SwarmMaster    bool
	SwarmHost      string
	SwarmDiscovery string
	ClusterNodes   []string
	storePath      string
}

func init() {
	drivers.Register("cluster", &drivers.RegisteredDriver{
		New:            NewDriver,
		GetCreateFlags: GetCreateFlags,
	})
}

// GetCreateFlags registers the flags this driver adds to
// "docker hosts create"
func GetCreateFlags() []cli.Flag {
	return []cli.Flag{
		cli.StringSliceFlag{
			Name:  "cluster-node",
			Usage: "Cluster node (machine name)",
			Value: &cli.StringSlice{},
		},
	}
}

func (d *Driver) getStore() *machine.Store {
	storePath := filepath.Dir(d.storePath)
	return machine.NewStore(storePath, d.CaCertPath, d.PrivateKeyPath)
}

func (d *Driver) getClusterNodes() ([]*machine.Machine, error) {
	nodes := []*machine.Machine{}

	st := d.getStore()
	for _, c := range d.ClusterNodes {
		m, err := st.Get(c)
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, m)
	}

	return nodes, nil
}

func NewDriver(machineName string, storePath string, caCert string, privateKey string) (drivers.Driver, error) {
	return &Driver{MachineName: machineName, storePath: storePath, CaCertPath: caCert, PrivateKeyPath: privateKey}, nil
}

func (d *Driver) DriverName() string {
	return "cluster"
}

func (d *Driver) SetConfigFromFlags(flags drivers.DriverOptions) error {
	d.SwarmMaster = flags.Bool("swarm-master")
	d.SwarmHost = flags.String("swarm-host")
	d.SwarmDiscovery = flags.String("swarm-discovery")
	d.ClusterNodes = flags.StringSlice("cluster-node")

	return nil
}

func (d *Driver) PreCreateCheck() error {
	return nil
}

func (d *Driver) Create() error {
	log.Infof("Created cluster...")
	return nil
}

func (d *Driver) GetURL() (string, error) {
	return "", nil
}

func (d *Driver) GetIP() (string, error) {
	return "", nil
}

func (d *Driver) GetState() (state.State, error) {
	s := state.Running

	nodes, err := d.getClusterNodes()
	if err != nil {
		return state.Error, err
	}

	for _, node := range nodes {
		mState, err := node.Driver.GetState()
		if err != nil {
			return state.Degraded, nil
		}

		if mState != state.Running {
			return state.Degraded, nil
		}
	}

	return s, nil
}

func nodeAction(m *machine.Machine, action string, wg *sync.WaitGroup) {
	actions := map[string]interface{}{
		"start":   m.Start,
		"stop":    m.Stop,
		"upgrade": m.Upgrade,
		"rm":      m.Remove,
	}

	if err := actions[action].(func() error)(); err != nil {
		log.Warnf("unable to %s node %s: %s", action, m.Name, err)
	}
	wg.Done()
}

func (d *Driver) clusterAction(action string) error {
	var wg sync.WaitGroup

	nodes, err := d.getClusterNodes()
	if err != nil {
		return err
	}

	for _, node := range nodes {
		mState, err := node.Driver.GetState()
		if err != nil {
			log.Warnf("unable to get state for node %s: %s", node, err)
			continue
		}

		switch action {
		case "start":
			if mState != state.Running {
				wg.Add(1)
				go nodeAction(node, action, &wg)
			}
		case "stop":
			if mState != state.Stopped {
				wg.Add(1)
				go nodeAction(node, action, &wg)
			}
		default:
			wg.Add(1)
			go nodeAction(node, action, &wg)
		}
	}

	wg.Wait()

	return nil
}

func (d *Driver) Start() error {
	return d.clusterAction("start")
}

func (d *Driver) Stop() error {
	return d.clusterAction("stop")
}

func (d *Driver) Remove() error {
	// TODO
	return nil
}

func (d *Driver) Restart() error {
	if err := d.clusterAction("stop"); err != nil {
		return err
	}

	if err := d.clusterAction("start"); err != nil {
		return err
	}

	return nil
}

func (d *Driver) Kill() error {
	// TODO
	return nil
}

func (d *Driver) StartDocker() error {
	//log.Debug("Starting Docker...")

	//cmd, err := d.GetSSHCommand("sudo service docker start")
	//if err != nil {
	//	return err
	//}
	//if err := cmd.Run(); err != nil {
	//	return err
	//}

	return nil
}

func (d *Driver) StopDocker() error {
	//log.Debug("Stopping Docker...")

	//cmd, err := d.GetSSHCommand("sudo service docker stop")
	//if err != nil {
	//	return err
	//}
	//if err := cmd.Run(); err != nil {
	//	return err
	//}

	return nil
}

func (d *Driver) GetDockerConfigDir() string {
	return dockerConfigDir
}

func (d *Driver) Upgrade() error {
	return d.clusterAction("upgrade")
}

func (d *Driver) GetSSHCommand(args ...string) (*exec.Cmd, error) {
	//return ssh.GetSSHCommand(d.IPAddress, 22, "root", d.sshKeyPath(), args...), nil
	return nil, nil
}

func (d *Driver) sshKeyPath() string {
	//return filepath.Join(d.storePath, "id_rsa")
	return ""
}

func (d *Driver) publicSSHKeyPath() string {
	//return d.sshKeyPath() + ".pub"
	return ""
}
