package config

// Project is the struct that represents a single project in the mirror.json config file
// These make up the bulk of the config file
type Project struct {
	// short is the key in the map, e.g. "debian"
	Short string

	Name        string `json:"name"`
	Color       string `json:"color"`
	Official    bool   `json:"official"`
	Page        string `json:"page"`
	HomePage    string `json:"homepage"`
	PublicRsync bool   `json:"publicRsync"`
	Icon        string `json:"icon"`
	Alternative string `json:"alternative"`
	Torrents    string `json:"torrents"`

	// SyncStyle isn't found in the file, instead it's inferred from the presence of "script", "rsync", or "static" keys
	SyncStyle string
	Script    *Script `json:"script"`
	Rsync     *Rsync  `json:"rsync"`
	Static    *Static `json:"static"`

	// ID is a unique identifier for the project
	ID uint8
}

// Rsync is the struct that represents a project that is synced with rsync
type Rsync struct {
	Stages       []string `json:"stages"`
	User         string   `json:"user"`
	Host         string   `json:"host"`
	Src          string   `json:"src"`
	Dest         string   `json:"dest"`
	SyncsPerDay  uint     `json:"syncs_per_day"`
	PasswordFile string   `json:"password_file"`
}

// Script is the struct that represents a project that is synced with a script
type Script struct {
	Env         map[string]string `json:"env"`
	Command     string            `json:"command"`
	Arguments   []string          `json:"arguments"`
	SyncsPerDay uint              `json:"syncs_per_day"`
}

// Static is the struct that represents a project that is never synced
type Static struct {
	Location    string `json:"location"`
	Source      string `json:"source"`
	Description string `json:"description"`
}
