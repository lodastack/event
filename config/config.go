package config

import (
	"io/ioutil"
	"os"
	"sync"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/coreos/etcd/client"
	"github.com/lodastack/log"
)

var (
	mux        = new(sync.RWMutex)
	config     = new(Config)
	configPath = ""
)

type Config struct {
	Com    CommonConfig   `toml:"common"`
	Reg    RegistryConfig `toml:"registry"`
	Etcd   EtcdConfig     `toml:"etcd"`
	Mail   MailConfig     `toml:"mail"`
	Sms    SmsConfig      `toml:"sms"`
	Wechat WechatConfig   `toml""wechat"`
	Log    LogConfig      `toml:"log"`
	Render RenderConfig   `toml:"render"`

	EtcdConfig client.Config `toml:"-"`
}
type EtcdConfig struct {
	Auth          bool          `toml:"auth"`
	Username      string        `toml:"username"`
	Password      string        `toml:"password"`
	Endpoints     []string      `toml:"endpoints"`
	HeaderTimeout time.Duration `toml:"timeout"`
	Path          string        `toml:"path"`
}
type MailConfig struct {
	User string `toml:"user"`
	Pwd  string `toml:"pwd"`
	Host string `toml:"host"`
	Port int    `toml:"port"`
	From string `toml:"from"`

	MailSuffix    string `toml:"mailsuffix"`
	SubjectPrefix string `toml:subjectprefix`
}

type RenderConfig struct {
	PhantomDir string `toml:"phantomdir"`
	ImgDir     string `toml:"imgdir"`
	RenderURL  string `toml:"renderurl"`
}

type SmsConfig struct {
	Script string `toml:script`
}

type WechatConfig struct {
	Script string `toml:script`
}

type CommonConfig struct {
	Listen             string `toml:"listen"`
	TopicsPollInterval int    `toml:"topicsPollInterval"`
	HiddenMetricSuffix string `toml:"hiddenMetricSuffix"`

	EventLogNs string `toml:"eventLogNs"`
}

type LogConfig struct {
	Enable   bool   `toml:"enable"`
	Path     string `toml:"path"`
	Level    string `toml:"level"`
	FileNum  int    `toml:"file_num"`
	FileSize int    `toml:"file_size"`
}

type RegistryConfig struct {
	Link      string `toml:"link"`
	ExpireDur int    `toml:"expireDur"`
}

func Reload() {
	err := LoadConfig(configPath)
	if err != nil {
		os.Exit(1)
	}
}

func LoadConfig(path string) (err error) {
	mux.Lock()
	defer mux.Unlock()
	configPath = path
	configFile, err := ioutil.ReadFile(path)
	if err != nil {
		log.Errorf("Error while loading config %s.\n%s\n", path, err.Error())
		return
	}
	if _, err = toml.Decode(string(configFile), &config); err != nil {
		log.Errorf("Error while decode the config %s.\n%s\n", path, err.Error())
		return
	} else {
		return nil
	}
}

func GetConfig() *Config {
	mux.RLock()
	defer mux.RUnlock()
	return config
}
