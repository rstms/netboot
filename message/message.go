package message

type NetbootConfig struct {
	Address           string `json:"address"`
	OS                string `json:"os"`
	Version           string `json:"version"`
	Hostname          string `json:"hostname"`
	Arch              string `json:"arch"`
	Serial            string `json:"serial"`
	Mirror            string `json:"mirror"`
	Response          string `json:"response"`
	DisklabelTemplate string `json:"disklabel_template"`
	KernelParams      string `json:"kernel_params"`
	Debug             bool   `json:"debug"`
	Quiet             bool   `json:"quiet"`
	Shutdown          bool   `json:"shutdown"`
	ImageSource       string `json:"image_source"`
	AlpineLoader      string `json:"alpine_loader"`
}
