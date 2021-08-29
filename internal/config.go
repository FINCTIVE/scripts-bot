package internal

var GlobalConfig Config

type Config struct {
	BotToken string   `yaml:"botToken"`
	Users    []string `yaml:"users"`
}
