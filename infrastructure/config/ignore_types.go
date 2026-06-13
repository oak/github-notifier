package config

import "github.com/oak/github-notifier/domain/pullrequest"

// ignoreConfigDTO is the yaml-annotated representation of ignore.yaml.
// It is private to this package; callers receive the mapped domain type.
type ignoreConfigDTO struct {
	Ignore struct {
		Global ignoreScopeDTO            `yaml:"global"`
		Repos  map[string]ignoreScopeDTO `yaml:",inline"`
	} `yaml:"ignore"`
}

type ignoreScopeDTO struct {
	Events     []string         `yaml:"events"`
	Except     []string         `yaml:"except"`
	Repos      []string         `yaml:"repos"`
	AuthoredBy []ignoreActorDTO `yaml:"authored_by"`
}

type ignoreActorDTO struct {
	Login  string   `yaml:"login"`
	Events []string `yaml:"events"`
	Except []string `yaml:"except"`
}

// toDomain maps the yaml DTO to the pure domain type.
func (d ignoreConfigDTO) toDomain() *pullrequest.IgnoreConfig {
	repos := make(map[string]pullrequest.IgnoreScope, len(d.Ignore.Repos))
	for name, scope := range d.Ignore.Repos {
		repos[name] = mapScope(scope)
	}
	cfg := &pullrequest.IgnoreConfig{}
	cfg.Ignore.Global = mapScope(d.Ignore.Global)
	cfg.Ignore.Repos = repos
	return cfg
}

func mapScope(s ignoreScopeDTO) pullrequest.IgnoreScope {
	actors := make([]pullrequest.IgnoreActorRule, len(s.AuthoredBy))
	for i, a := range s.AuthoredBy {
		actors[i] = pullrequest.IgnoreActorRule{
			Login:  a.Login,
			Events: a.Events,
			Except: a.Except,
		}
	}
	return pullrequest.IgnoreScope{
		Events:     s.Events,
		Except:     s.Except,
		Repos:      s.Repos,
		AuthoredBy: actors,
	}
}
