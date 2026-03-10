package sandbox

type Config struct {
	Image           string                 `mapstructure:"image"`
	Port            int                    `mapstructure:"port"`
	ProvisionerURL  string                 `mapstructure:"provisioner_url"`
	AutoStart       bool                   `mapstructure:"auto_start"`
	ContainerPrefix string                 `mapstructure:"container_prefix"`
	IdleTimeout     int                    `mapstructure:"idle_timeout"`
	Mounts          []*VolumeMount         `mapstructure:"mounts"`
	Environment     map[string]string      `mapstructure:"environment"`
	Extra           map[string]interface{} `mapstructure:"extra"`
	BaseDir         string                 `mapstructure:"base_dir"`
}

type VolumeMount struct {
	HostPath      string `mapstructure:"host_path"`
	ContainerPath string `mapstructure:"container_path"`
	ReadOnly      bool   `mapstructure:"read_only"`
}
