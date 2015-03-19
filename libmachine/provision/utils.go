package provision

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/url"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/machine/libmachine/auth"
	"github.com/docker/machine/libmachine/provision/pkgaction"
	"github.com/docker/machine/libmachine/swarm"
	"github.com/docker/machine/utils"
)

type DockerConfig struct {
	EngineConfig     string
	EngineConfigPath string
}

func installDockerGeneric(p Provisioner) error {
	// install docker - until cloudinit we use ubuntu everywhere so we
	// just install it using the docker repos
	cmd, err := p.SSHCommand("if [ ! -e /usr/bin/docker && ! -e /usr/local/bin/docker ]; then curl -sSL https://get.docker.com | sh -; fi")
	if err != nil {
		return err
	}

	// HACK: the script above will output debug to stderr; we save it and
	// then check if the command returned an error; if so, we show the debug
	var buf bytes.Buffer
	cmd.Stderr = &buf

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("error installing docker: %s\n%s\n", err, string(buf.Bytes()))
	}

	return nil
}

func ConfigureAuth(p Provisioner, authConfig auth.AuthOptions) error {
	var (
		err error
	)

	machineName := p.GetDriver().GetMachineName()

	// copy certs to client dir for docker client
	machineDir := filepath.Join(utils.GetMachineDir(), machineName)
	if err := utils.CopyFile(authConfig.CaCertPath, filepath.Join(machineDir, "ca.pem")); err != nil {
		log.Fatalf("Error copying ca.pem to machine dir: %s", err)
	}

	clientCertPath := filepath.Join(utils.GetMachineCertDir(), "cert.pem")
	if err := utils.CopyFile(clientCertPath, filepath.Join(machineDir, "cert.pem")); err != nil {
		log.Fatalf("Error copying cert.pem to machine dir: %s", err)
	}

	clientKeyPath := filepath.Join(utils.GetMachineCertDir(), "key.pem")
	if err := utils.CopyFile(clientKeyPath, filepath.Join(machineDir, "key.pem")); err != nil {
		log.Fatalf("Error copying key.pem to machine dir: %s", err)
	}

	authConfig.ServerCertPath = filepath.Join(authConfig.StorePath, "server.pem")
	authConfig.ServerKeyPath = filepath.Join(authConfig.StorePath, "server-key.pem")

	org := machineName
	bits := 2048

	log.Debugf("generating server cert: %s ca-key=%s private-key=%s org=%s",
		authConfig.ServerCertPath,
		authConfig.CaCertPath,
		authConfig.PrivateKeyPath,
		org,
	)

	ip, err := p.GetDriver().GetIP()
	if err != nil {
		return err
	}

	// TODO: Switch to passing just authConfig to this func
	// instead of all these individual fields
	err = utils.GenerateCert(
		[]string{ip},
		authConfig.ServerCertPath,
		authConfig.ServerKeyPath,
		authConfig.CaCertPath,
		authConfig.PrivateKeyPath,
		org,
		bits,
	)
	if err != nil {
		return fmt.Errorf("error generating server cert: %s", err)
	}

	if err := p.Service("docker", pkgaction.Stop); err != nil {
		return err
	}

	dockerDir := p.GetDockerConfigDir()

	cmd, err := p.SSHCommand(fmt.Sprintf("sudo mkdir -p %s", dockerDir))
	if err != nil {
		return err
	}
	if err := cmd.Run(); err != nil {
		return err
	}

	// upload certs and configure TLS auth
	caCert, err := ioutil.ReadFile(authConfig.CaCertPath)
	if err != nil {
		return err
	}

	// due to windows clients, we cannot use filepath.Join as the paths
	// will be mucked on the linux hosts
	machineCaCertPath := path.Join(dockerDir, "ca.pem")
	authConfig.CaCertRemotePath = machineCaCertPath

	serverCert, err := ioutil.ReadFile(authConfig.ServerCertPath)
	if err != nil {
		return err
	}
	machineServerCertPath := path.Join(dockerDir, "server.pem")
	authConfig.ServerCertRemotePath = machineServerCertPath

	serverKey, err := ioutil.ReadFile(authConfig.ServerKeyPath)
	if err != nil {
		return err
	}
	machineServerKeyPath := path.Join(dockerDir, "server-key.pem")
	authConfig.ServerKeyRemotePath = machineServerKeyPath

	cmd, err = p.SSHCommand(fmt.Sprintf("echo \"%s\" | sudo tee %s", string(caCert), machineCaCertPath))
	if err != nil {
		return err
	}
	if err := cmd.Run(); err != nil {
		return err
	}

	cmd, err = p.SSHCommand(fmt.Sprintf("echo \"%s\" | sudo tee %s", string(serverKey), machineServerKeyPath))
	if err != nil {
		return err
	}
	if err := cmd.Run(); err != nil {
		return err
	}

	cmd, err = p.SSHCommand(fmt.Sprintf("echo \"%s\" | sudo tee %s", string(serverCert), machineServerCertPath))
	if err != nil {
		return err
	}
	if err := cmd.Run(); err != nil {
		return err
	}

	dockerUrl, err := p.GetDriver().GetURL()
	if err != nil {
		return err
	}
	u, err := url.Parse(dockerUrl)
	if err != nil {
		return err
	}
	dockerPort := 2376
	parts := strings.Split(u.Host, ":")
	if len(parts) == 2 {
		dPort, err := strconv.Atoi(parts[1])
		if err != nil {
			return err
		}
		dockerPort = dPort
	}

	dkrcfg, err := p.GenerateDockerConfig(dockerPort, authConfig)
	if err != nil {
		return err
	}

	cmd, err = p.SSHCommand(fmt.Sprintf("echo \"%s\" | sudo tee -a %s", dkrcfg.EngineConfig, dkrcfg.EngineConfigPath))
	if err != nil {
		return err
	}
	if err := cmd.Run(); err != nil {
		return err
	}

	if err := p.Service("docker", pkgaction.Start); err != nil {
		return err
	}

	return nil
}

func getDefaultDaemonOpts(driverName string, authConfig auth.AuthOptions) string {
	return fmt.Sprintf(`--tlsverify --tlscacert=%s --tlskey=%s --tlscert=%s %s`,
		authConfig.CaCertRemotePath,
		authConfig.ServerKeyRemotePath,
		authConfig.ServerCertRemotePath,
		fmt.Sprintf("--label=provider=%s", driverName),
	)
}

func configureSwarm(p Provisioner, swarmConfig swarm.SwarmOptions) error {
	if !swarmConfig.IsSwarm {
		return nil
	}

	basePath := p.GetDockerConfigDir()

	tlsCaCert := path.Join(basePath, "ca.pem")
	tlsCert := path.Join(basePath, "server.pem")
	tlsKey := path.Join(basePath, "server-key.pem")
	masterArgs := fmt.Sprintf("--tlsverify --tlscacert=%s --tlscert=%s --tlskey=%s -H %s %s",
		tlsCaCert, tlsCert, tlsKey, swarmConfig.Host, swarmConfig.Discovery)
	nodeArgs := fmt.Sprintf("--addr %s %s", swarmConfig.Addr, swarmConfig.Discovery)

	u, err := url.Parse(swarmConfig.Host)
	if err != nil {
		return err
	}

	parts := strings.Split(u.Host, ":")
	port := parts[1]

	if err := utils.WaitForDocker(swarmConfig.Addr); err != nil {
		return err
	}

	cmd, err := p.SSHCommand(fmt.Sprintf("sudo docker pull %s", swarm.DockerImage))
	if err != nil {
		return err
	}
	if err := cmd.Run(); err != nil {
		return err
	}

	dockerDir := p.GetDockerConfigDir()

	// if master start master agent
	if swarmConfig.Master {
		log.Debug("launching swarm master")
		log.Debugf("master args: %s", masterArgs)
		cmd, err = p.SSHCommand(fmt.Sprintf("sudo docker run -d -p %s:%s --restart=always --name swarm-agent-master -v %s:%s %s manage %s",
			port, port, dockerDir, dockerDir, swarm.DockerImage, masterArgs))
		if err != nil {
			return err
		}
		if err := cmd.Run(); err != nil {
			return err
		}
	}

	// start node agent
	log.Debug("launching swarm node")
	log.Debugf("node args: %s", nodeArgs)
	cmd, err = p.SSHCommand(fmt.Sprintf("sudo docker run -d --restart=always --name swarm-agent -v %s:%s %s join %s",
		dockerDir, dockerDir, swarm.DockerImage, nodeArgs))
	if err != nil {
		return err
	}
	if err := cmd.Run(); err != nil {
		return err
	}

	return nil
}
