// Package config parses JSON configuration files and exports the Config struct
// for server-side use and the ClientConfig struct, for JSON stringification and
// passing to the  client,
package config

import (
	"encoding/json"
	"github.com/Soreil/mnemonics"
	"github.com/bakape/meguca/util"
	"io/ioutil"
	"path/filepath"
)

// Overridable path for tests
var configRoot = "config"

// Server stores the global configuration. It is loaded only once
// during start up and considered implicitly immutable during the rest of
// runtime.
type Server struct {
	HTTP struct {
		Addr, Origin, Cert, Key string
		SSL, TrustProxies, Gzip bool
	}
	Rethinkdb struct {
		Addr, Db string
	}
	Boards struct {
		Enabled []string
		Boards  map[string]struct {
			MaxThreads, MaxBump int
			Title               string
		}
		Default, Staff string
		Psuedo, Links  [][2]string
		Prune          bool
	}
	Lang struct {
		Enabled []string
		Default string
	}
	Staff struct {
		Classes     map[string]StaffClass
		Keyword     string
		SessionTime int
	}
	Images struct {
		Max struct {
			Size, Width, Height, Pixels int64
		}
		JpegQuality        uint8
		PngQuality         string
		WebmAudio          bool
		Hats               bool
		DuplicateThreshold uint8
		Spoilers           []uint8
		Formats            map[string]bool
	}
	Posts struct {
		Salt, ExcludeRegex                       string
		ThreadCreationCooldown, MaxSubjectLength int
		ReadOnly, SageEnabled, ForcedAnon        bool
	}
	Recaptcha struct {
		Public, Private string
	}
	Banners, FAQ, Eightball                                        []string
	Radio, Pyu, IllyaDance                                         bool
	FeedbackEmail, DefaultCSS, Frontpage, InfoBanner, InjectJSPath string
}

// StaffClass contains properties of a single staff personel type
type StaffClass struct {
	Alias   string
	Members map[string]string
	Rights  map[string]bool
}

// Config contains currently loaded server configuration
var Config Server

// client is a subset of serverConfigs, that is exported as JSON to all clients
type client struct {
	Boards struct {
		Enabled []string `json:"enabled"`
		Boards  map[string]struct {
			Title string `json:"title"`
		} `json:"boards"`
		Default string      `json:"default"`
		Psuedo  [][2]string `json:"psuedo"`
		Links   [][2]string `json:"links"`
	} `json:"boards"`
	Lang struct {
		Enabled []string `json:"enabled"`
		Default string   `json:"default"`
	} `json:"lang"`
	Staff struct {
		Classes map[string]struct {
			Alias  string          `json:"alias"`
			Rights map[string]bool `json:"rights"`
		} `json:"classes"`
		Keyword string `json:"keyword"`
	} `json:"staff"`
	Images struct {
		thumb struct {
			ThumbDims [2]int `json:"thumbDims"`
			MidDims   [2]int `json:"midDims"`
		}
		Spoilers []int `json:"spoilers"`
		Hats     bool  `json:"hats"`
	} `json:"images"`
	Banners       []string `json:"banners"`
	FAQ           []string `json:"FAQ"`
	Eightball     []string `json:"eightball"`
	Radio         bool     `json:"radio"`
	IllyaDance    bool     `json:"illiyaDance"`
	FeedbackEmail string   `json:"feedbackEmail"`
	DefaultCSS    string   `json:"defaultCSS"`
	InfoBanner    string   `json:"infoBanner"`
}

// ClientConfig exports public settings all clients can access
var ClientConfig []byte

// Hash stores the truncated MD5 hash of Config
var Hash string

// LoadConfig reads and parses the JSON config file
func LoadConfig() error {
	path := filepath.FromSlash(configRoot + "/config.json")
	file, err := ioutil.ReadFile(path)
	if err != nil {
		return util.WrapError("Error reading configuration file", err)
	}

	if err := json.Unmarshal(file, &Config); err != nil {
		return parseError(err)
	}

	var data client
	if err := json.Unmarshal(file, &data); err != nil {
		return parseError(err)
	}

	clientJSON, err := json.Marshal(data)
	if err != nil {
		return parseError(err)
	}
	ClientConfig = clientJSON
	hash, err := util.HashBuffer(file)
	if err != nil {
		return parseError(err)
	}
	Hash = hash
	if err := mnemonic.SetSalt(Config.Posts.Salt); err != nil {
		return err
	}
	return nil
}

func parseError(err error) error {
	return util.WrapError("Error parsing configuration file", err)
}
