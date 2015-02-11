package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"

	log "github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"

	"github.com/docker/machine/drivers"
	_ "github.com/docker/machine/drivers/amazonec2"
	//_ "github.com/docker/machine/drivers/azure"
	_ "github.com/docker/machine/drivers/digitalocean"
	//_ "github.com/docker/machine/drivers/google"
	//_ "github.com/docker/machine/drivers/hyperv"
	//_ "github.com/docker/machine/drivers/none"
	//_ "github.com/docker/machine/drivers/openstack"
	//_ "github.com/docker/machine/drivers/rackspace"
	//_ "github.com/docker/machine/drivers/softlayer"
	//_ "github.com/docker/machine/drivers/virtualbox"
	//_ "github.com/docker/machine/drivers/vmwarefusion"
	//_ "github.com/docker/machine/drivers/vmwarevcloudair"
	//_ "github.com/docker/machine/drivers/vmwarevsphere"
	"github.com/docker/machine/state"
	"github.com/docker/machine/store"
	"github.com/docker/machine/utils"
)

type machineConfig struct {
	caCertPath     string
	clientCertPath string
	clientKeyPath  string
	machineUrl     string
}

type hostListItem struct {
	Name       string
	Active     bool
	DriverName string
	State      state.State
	URL        string
}

type hostListItemByName []hostListItem

func (h hostListItemByName) Len() int {
	return len(h)
}

func (h hostListItemByName) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
}

func (h hostListItemByName) Less(i, j int) bool {
	return strings.ToLower(h[i].Name) < strings.ToLower(h[j].Name)
}

func getCreateCommands() []cli.Command {
	// inject cmdCreate action
	commands := drivers.GetCommands()
	newCommands := []cli.Command{}
	for _, c := range commands {
		c.Action = cmdCreate
		newCommands = append(newCommands, c)
	}

	return newCommands
}

var Commands = []cli.Command{
	{
		Name:   "active",
		Usage:  "Get or set the active machine",
		Action: cmdActive,
	},
	{
		Name:        "create",
		Usage:       "Create a Machine",
		Subcommands: getCreateCommands(),
	},
	{
		Name:   "config",
		Usage:  "Print the connection config for machine",
		Action: cmdConfig,
	},
	{
		Name:   "inspect",
		Usage:  "Inspect information about a machine",
		Action: cmdInspect,
	},
	{
		Name:   "ip",
		Usage:  "Get the IP address of a machine",
		Action: cmdIp,
	},
	{
		Name:   "kill",
		Usage:  "Kill a machine",
		Action: cmdKill,
	},
	{
		Flags: []cli.Flag{
			cli.BoolFlag{
				Name:  "quiet, q",
				Usage: "Enable quiet mode",
			},
		},
		Name:   "ls",
		Usage:  "List machines",
		Action: cmdLs,
	},
	{
		Name:   "restart",
		Usage:  "Restart a machine",
		Action: cmdRestart,
	},
	{
		Flags: []cli.Flag{
			cli.BoolFlag{
				Name:  "force, f",
				Usage: "Remove local configuration even if machine cannot be removed",
			},
		},
		Name:   "rm",
		Usage:  "Remove a machine",
		Action: cmdRm,
	},
	{
		Name:   "env",
		Usage:  "Display the commands to set up the environment for the Docker client",
		Action: cmdEnv,
	},
	{
		Name:   "ssh",
		Usage:  "Log into or run a command on a machine with SSH",
		Action: cmdSsh,
	},
	{
		Name:   "start",
		Usage:  "Start a machine",
		Action: cmdStart,
	},
	{
		Name:   "stop",
		Usage:  "Stop a machine",
		Action: cmdStop,
	},
	{
		Name:   "upgrade",
		Usage:  "Upgrade a machine to the latest version of Docker",
		Action: cmdUpgrade,
	},
	{
		Name:   "url",
		Usage:  "Get the URL of a machine",
		Action: cmdUrl,
	},
}

func cmdActive(c *cli.Context) {
	name := c.Args().First()
	st := store.NewStore(c.GlobalString("storage-path"), c.GlobalString("tls-ca-cert"), c.GlobalString("tls-ca-key"))

	if name == "" {
		host, err := st.GetActive()
		if err != nil {
			log.Fatalf("error getting active host: %v", err)
		}
		if host != nil {
			fmt.Println(host.Name)
		}
	} else if name != "" {
		host, err := st.Load(name)
		if err != nil {
			log.Fatalf("error loading host: %v", err)
		}

		if err := st.SetActive(host); err != nil {
			log.Fatalf("error setting active host: %v", err)
		}
	} else {
		cli.ShowCommandHelp(c, "active")
	}
}

func cmdConfig(c *cli.Context) {
	cfg, err := getMachineConfig(c)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("--tls --tlscacert=%s --tlscert=%s --tlskey=%s -H=%q",
		cfg.caCertPath, cfg.clientCertPath, cfg.clientKeyPath, cfg.machineUrl)
}

func cmdInspect(c *cli.Context) {
	prettyJSON, err := json.MarshalIndent(getHost(c), "", "    ")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(string(prettyJSON))
}

func cmdIp(c *cli.Context) {
	ip, err := getHost(c).Driver.GetIP()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(ip)
}

func cmdKill(c *cli.Context) {
	if err := getHost(c).Driver.Kill(); err != nil {
		log.Fatal(err)
	}
}

func cmdLs(c *cli.Context) {
	quiet := c.Bool("quiet")
	st := store.NewStore(c.GlobalString("storage-path"), c.GlobalString("tls-ca-cert"), c.GlobalString("tls-ca-key"))

	hostList, err := st.List()
	if err != nil {
		log.Fatal(err)
	}

	w := tabwriter.NewWriter(os.Stdout, 5, 1, 3, ' ', 0)

	if !quiet {
		fmt.Fprintln(w, "NAME\tACTIVE\tDRIVER\tSTATE\tURL")
	}

	items := []hostListItem{}
	hostListItems := make(chan hostListItem)

	for _, host := range hostList {
		if !quiet {
			tmpHost, err := st.GetActive()
			if err != nil {
				log.Errorf("There's a problem with the active host: %s", err)
			}

			if tmpHost == nil {
				log.Errorf("There's a problem finding the active host")
			}

			go getHostState(host, *st, hostListItems)
		} else {
			fmt.Fprintf(w, "%s\n", host.Name)
		}
	}

	if !quiet {
		for i := 0; i < len(hostList); i++ {
			items = append(items, <-hostListItems)
		}
	}

	close(hostListItems)

	sort.Sort(hostListItemByName(items))

	for _, item := range items {
		activeString := ""
		if item.Active {
			activeString = "*"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			item.Name, activeString, item.DriverName, item.State, item.URL)
	}

	w.Flush()
}

func cmdRestart(c *cli.Context) {
	if err := getHost(c).Driver.Restart(); err != nil {
		log.Fatal(err)
	}
}

func cmdRm(c *cli.Context) {
	if len(c.Args()) == 0 {
		cli.ShowCommandHelp(c, "rm")
		log.Fatal("You must specify a machine name")
	}

	force := c.Bool("force")

	isError := false

	st := store.NewStore(c.GlobalString("storage-path"), c.GlobalString("tls-ca-cert"), c.GlobalString("tls-ca-key"))
	for _, host := range c.Args() {
		if err := st.Remove(host, force); err != nil {
			log.Errorf("Error removing machine %s: %s", host, err)
			isError = true
		}
	}
	if isError {
		log.Fatal("There was an error removing a machine. To force remove it, pass the -f option. Warning: this might leave it running on the provider.")
	}
}

func cmdEnv(c *cli.Context) {
	cfg, err := getMachineConfig(c)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("export DOCKER_TLS_VERIFY=yes\nexport DOCKER_CERT_PATH=%s\nexport DOCKER_HOST=%s\n",
		utils.GetMachineClientCertDir(), cfg.machineUrl)
}

func cmdSsh(c *cli.Context) {
	var (
		err    error
		sshCmd *exec.Cmd
	)
	name := c.Args().First()
	st := store.NewStore(c.GlobalString("storage-path"), c.GlobalString("tls-ca-cert"), c.GlobalString("tls-ca-key"))

	if name == "" {
		host, err := st.GetActive()
		if err != nil {
			log.Fatalf("unable to get active host: %v", err)
		}

		name = host.Name
	}

	host, err := st.Load(name)
	if err != nil {
		log.Fatal(err)
	}

	if len(c.Args()) <= 1 {
		sshCmd, err = host.Driver.GetSSHCommand()
	} else {
		sshCmd, err = host.Driver.GetSSHCommand(c.Args()[1:]...)
	}
	if err != nil {
		log.Fatal(err)
	}

	sshCmd.Stdin = os.Stdin
	sshCmd.Stdout = os.Stdout
	sshCmd.Stderr = os.Stderr
	if err := sshCmd.Run(); err != nil {
		log.Fatal(err)
	}
}

func cmdStart(c *cli.Context) {
	if err := getHost(c).Start(); err != nil {
		log.Fatal(err)
	}
}

func cmdStop(c *cli.Context) {
	if err := getHost(c).Stop(); err != nil {
		log.Fatal(err)
	}
}

func cmdUpgrade(c *cli.Context) {
	if err := getHost(c).Upgrade(); err != nil {
		log.Fatal(err)
	}
}

func cmdUrl(c *cli.Context) {
	url, err := getHost(c).GetURL()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(url)
}

func cmdNotFound(c *cli.Context, command string) {
	log.Fatalf(
		"%s: '%s' is not a %s command. See '%s --help'.",
		c.App.Name,
		command,
		c.App.Name,
		c.App.Name,
	)
}

func getHost(c *cli.Context) *store.Host {
	name := c.Args().First()
	st := store.NewStore(c.GlobalString("storage-path"), c.GlobalString("tls-ca-cert"), c.GlobalString("tls-ca-key"))

	if name == "" {
		host, err := st.GetActive()
		if err != nil {
			log.Fatalf("unable to get active host: %v", err)
		}

		if host == nil {
			log.Fatal("unable to get active host, active file not found")
		}
		return host
	}

	host, err := st.Load(name)
	if err != nil {
		log.Fatalf("unable to load host: %v", err)
	}
	return host
}

func getHostState(host store.Host, st store.Store, hostListItems chan<- hostListItem) {
	currentState, err := host.Driver.GetState()
	if err != nil {
		log.Errorf("error getting state for host %s: %s", host.Name, err)
	}

	url, err := host.GetURL()
	if err != nil {
		if err == drivers.ErrHostIsNotRunning {
			url = ""
		} else {
			log.Errorf("error getting URL for host %s: %s", host.Name, err)
		}
	}

	isActive, err := st.IsActive(&host)
	if err != nil {
		log.Debugf("error determining whether host %q is active: %s",
			host.Name, err)
	}

	hostListItems <- hostListItem{
		Name:       host.Name,
		Active:     isActive,
		DriverName: host.Driver.DriverName(),
		State:      currentState,
		URL:        url,
	}
}

func getMachineConfig(c *cli.Context) (*machineConfig, error) {
	name := c.Args().First()
	st := store.NewStore(c.GlobalString("storage-path"), c.GlobalString("tls-ca-cert"), c.GlobalString("tls-ca-key"))
	var machine *store.Host

	if name == "" {
		m, err := st.GetActive()
		if err != nil {
			log.Fatalf("error getting active host: %v", err)
		}
		if m == nil {
			return nil, fmt.Errorf("There is no active host")
		}
		machine = m
	} else {
		m, err := st.Load(name)
		if err != nil {
			return nil, fmt.Errorf("Error loading machine config: %s", err)
		}
		machine = m
	}

	caCert := filepath.Join(utils.GetMachineClientCertDir(), "ca.pem")
	clientCert := filepath.Join(utils.GetMachineClientCertDir(), "cert.pem")
	clientKey := filepath.Join(utils.GetMachineClientCertDir(), "key.pem")
	machineUrl, err := machine.GetURL()
	if err != nil {
		if err == drivers.ErrHostIsNotRunning {
			machineUrl = ""
		} else {
			return nil, fmt.Errorf("Unexpected error getting machine url: %s", err)
		}
	}
	return &machineConfig{
		caCertPath:     caCert,
		clientCertPath: clientCert,
		clientKeyPath:  clientKey,
		machineUrl:     machineUrl,
	}, nil
}

func cmdCreate(c *cli.Context) {
	driver := c.Command.Name
	name := c.Args().First()

	if name == "" {
		cli.ShowCommandHelp(c, c.Command.Name)
		log.Fatal("You must specify a machine name")
	}

	if err := utils.SetupMachineCertificates(c.GlobalString("tls-ca-cert"), c.GlobalString("tls-ca-key"),
		c.GlobalString("tls-client-cert"), c.GlobalString("tls-client-key")); err != nil {
		log.Fatalf("Error configuring certificates: %s", err)
	}

	st := store.NewStore(c.GlobalString("storage-path"), c.GlobalString("tls-ca-cert"), c.GlobalString("tls-ca-key"))

	host, err := st.Create(name, driver, c)
	if err != nil {
		log.Errorf("Error creating machine: %s", err)
		log.Warn("You will want to check the provider to make sure the machine and associated resources were properly removed.")
		log.Fatal("Error creating machine")
	}
	if err := st.SetActive(host); err != nil {
		log.Fatalf("error setting active host: %v", err)
	}

	log.Infof("%q has been created and is now the active machine.", name)
	log.Infof("To point your Docker client at it, run this in your shell: $(%s env %s)", c.App.Name, name)
}
