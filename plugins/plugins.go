package plugins

type DriverOpts struct {
	Data map[string]interface{}
}

func (d DriverOpts) String(key string) string {
	return d.Data[key].(string)
}

func (d DriverOpts) StringSlice(key string) []string {
	return d.Data[key].([]string)
}

func (d DriverOpts) Int(key string) int {
	return d.Data[key].(int)
}

func (d DriverOpts) Bool(key string) bool {
	return d.Data[key].(bool)
}

type PluginResponse struct {
	Data interface{}
}

type PluginOptions struct {
	MachineName   string
	StorePath     string
	CaCertPath    string
	CaKeyPath     string
	DriverOptions map[string]interface{}
}
