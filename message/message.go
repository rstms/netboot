package message

type NetbootConfig struct {
	Address           string `json:"address"`
	OS                string `json:"os"`
	Version           string `json:"version"`
	Hostname          string `json:"hostname"`
	Arch              string `json:"arch"`
	Serial            string `json:"serial"`
	EgressInterface   string `json:"egress_interface"`
	RootDevice        string `json:"root_device"`
	Mirror            string `json:"mirror"`
	Response          string `json:"response"`
	DisklabelTemplate string `json:"disklabel_template"`
	KernelParams      string `json:"kernel_params"`
	Debug             bool   `json:"debug"`
	Quiet             bool   `json:"quiet"`
	Shutdown          bool   `json:"shutdown"`
	ImageSource       string `json:"image_source"`
	AlpineLoader      string `json:"alpine_loader"`
	BootstrapId       string `json:"bootstrap_id"`
}

type HostState struct {
	MAC         string `json:"mac"`
	IP          string `json:"ip"`
	State       string `json:"state"`
	BootstrapId string `json:"bootstrap_id"`
}

type NetbootResponse struct {
	Message string `json:"message"`
}

type NetbootAddHostResponse struct {
	Message   string   `json:"message"`
	ISO       string   `json: "iso"`
	IsoSHA512 string   `json: "iso_sha512"`
	Files     []string `json:"files"`
}

type NetbootListHostsResponse struct {
	Message   string   `json:"message"`
	Addresses []string `json:"addresses"`
}

type NetbootHostStatusResponse struct {
	Message string    `json:"message"`
	Status  HostState `json:"status"`
}

type NetbootDeleteHostResponse struct {
	Message string   `json:"message"`
	Files   []string `json:"files"`
}
