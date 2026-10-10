package cli

import "botropolis.city/botropolis/pkg/config"

type Loader func() (*config.Config, error)

type Services interface {
	Status(cfg *config.Config) StatusRunner
	Hooks(cfg *config.Config) HooksRunner
	Sessions(cfg *config.Config) SessionsRunner
}

type CityServices interface {
	City(cfg *config.Config) CityRunner
}
