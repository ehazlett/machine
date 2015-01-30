package drivers

type MachineOptions struct {
	Host           string
	Labels         []string
	CaCertPath     string
	ServerKeyPath  string
	ServerCertPath string
}
