package authorization

import (
	"errors"
	"os"
	"strings"

	yaml "gopkg.in/yaml.v3"
)

// Config is the root authorization configuration loaded from authorization.yaml
type Config struct {
	Coarse    CoarseConfig    `yaml:"coarse-check"`
	FineGrain FineGrainConfig `yaml:"finegrain-check"`
}

type CoarseConfig struct {
	Enabled          bool              `yaml:"enabled"`
	AnonymousAccess  bool              `yaml:"anonymous-access"`
	ValidationURL    string            `yaml:"validation-url"`
	ClientID         string            `yaml:"client-id"`
	ClientSecret     string            `yaml:"client-secret"`
	ClientAuthMethod string            `yaml:"client-auth-method"`
	ResourceMap      map[string]string `yaml:"resource-map"`
}

type FineRule struct {
	Roles       []string          `yaml:"roles"`
	RulesetName string            `yaml:"ruleset-name"`
	RulesetID   string            `yaml:"ruleset-id"`
	Body        map[string]string `yaml:"body"`
}

type FineGrainConfig struct {
	Enabled          bool                `yaml:"enabled"`
	ValidationURL    string              `yaml:"validation-url"`
	ClientID         string              `yaml:"client-id"`
	ClientSecret     string              `yaml:"client-secret"`
	ClientAuthMethod string              `yaml:"client-auth-method"`
	ResourceMap      map[string]FineRule `yaml:"resource-map"`
}

var cfg *Config

// Load reads YAML config from the given path and stores it globally for use by checks
func Load(path string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var c Config
	if err := yaml.Unmarshal(b, &c); err != nil {
		return err
	}
	// Validate at least one section enabled with a URL
	coarseOK := c.Coarse.Enabled && strings.TrimSpace(c.Coarse.ValidationURL) != ""
	fineOK := c.FineGrain.Enabled && strings.TrimSpace(c.FineGrain.ValidationURL) != ""
	if !coarseOK && !fineOK {
		return errors.New("authorization: at least one enabled section with validation-url is required")
	}
	cfg = &c
	return nil
}

// ConfigOrNil returns the loaded config or nil if not loaded.
func ConfigOrNil() *Config { return cfg }

// helper: match coarse resource-map key against a path and return the mapped resource
func (c CoarseConfig) MatchResource(path string) (string, bool) {
	bestKey := ""
	bestSpecificity := -1
	for k := range c.ResourceMap {
		pattern := normalizePattern(k)
		if pm, has := splitMethod(pattern); has {
			// coarse patterns ignore method suffix
			pattern = pm.pattern
		}
		if matched, spec := pathMatch(pattern, path); matched {
			if spec > bestSpecificity {
				bestSpecificity = spec
				bestKey = k
			}
		}
	}
	if bestKey == "" {
		return "", false
	}
	return c.ResourceMap[bestKey], true
}

// helper: match fine-grain rule by method and path
func (f FineGrainConfig) MatchRule(method, path string) (FineRule, bool) {
	method = strings.ToUpper(method)
	bestKey := ""
	bestSpecificity := -1
	for k := range f.ResourceMap {
		p := normalizePattern(k)
		pm, hasMethod := splitMethod(p)
		if hasMethod && pm.method != method {
			continue
		}
		if matched, spec := pathMatch(pm.pattern, path); matched {
			if spec > bestSpecificity {
				bestSpecificity = spec
				bestKey = k
			}
		}
	}
	if bestKey == "" {
		return FineRule{}, false
	}
	return f.ResourceMap[bestKey], true
}

// normalizePattern trims surrounding [ ] if present
func normalizePattern(raw string) string {
	s := strings.TrimSpace(raw)
	if strings.HasPrefix(s, "[") && strings.HasSuffix(s, "]") {
		s = strings.TrimPrefix(s, "[")
		s = strings.TrimSuffix(s, "]")
	}
	return s
}

type patternMethod struct {
	pattern string
	method  string
}

func splitMethod(p string) (patternMethod, bool) {
	// pattern may be like /path/**:POST
	if i := strings.LastIndex(p, ":"); i != -1 {
		return patternMethod{pattern: p[:i], method: strings.ToUpper(strings.TrimSpace(p[i+1:]))}, true
	}
	return patternMethod{pattern: p}, false
}

// pathMatch supports '*', '**' wildcards. Returns matched and a specificity score (higher is more specific)
func pathMatch(pattern, path string) (bool, int) {
	// quick exact match
	if pattern == path {
		return true, len(path) + 1000
	}
	// split by '/'
	ps := strings.Split(strings.TrimPrefix(pattern, "/"), "/")
	ss := strings.Split(strings.TrimPrefix(path, "/"), "/")
	i, j := 0, 0
	specificity := 0
	for i < len(ps) {
		if ps[i] == "**" {
			// match rest
			specificity += 1
			return true, specificity
		}
		if j >= len(ss) {
			return false, 0
		}
		switch ps[i] {
		case "*":
			// matches exactly one segment, low specificity
			specificity += 1
			i++
			j++
		default:
			if ps[i] != ss[j] {
				return false, 0
			}
			specificity += 5 // literal segment is more specific
			i++
			j++
		}
	}
	if j != len(ss) {
		return false, 0
	}
	return true, specificity
}
