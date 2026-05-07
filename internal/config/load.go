package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"unicode"

	"github.com/creasty/defaults"
	"github.com/go-viper/mapstructure/v2"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
	"github.com/openkcm/common-sdk/pkg/commoncfg"
)

type koanfSetter interface {
	setKoanf(ko *koanf.Koanf)
}

const configFile = "config.yaml"

var koanfUnmarshalConf = koanf.UnmarshalConf{
	Tag: "yaml",
	DecoderConfig: &mapstructure.DecoderConfig{
		DecodeHook: mapstructure.ComposeDecodeHookFunc(
			mapstructure.StringToTimeDurationHookFunc(),
			mapstructure.TextUnmarshallerHookFunc()),
		Metadata:         nil,
		WeaklyTypedInput: true,
		SquashTagOption:  "inline",
	},
}

func Load(buildInfo string, paths ...string) (*Config, error) {
	for i, path := range paths {
		paths[i] = filepath.Join(path, configFile)
	}

	ko := koanf.New(".")
	var loaded bool
	for _, path := range paths {
		if err := ko.Load(file.Provider(path), yamlParser{}); err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				return nil, fmt.Errorf("loading configuration from file %s: %w", path, err)
			}
		} else {
			loaded = true
		}
	}

	if !loaded {
		return nil, fmt.Errorf("no config file found at the paths %q: %w", strings.Join(paths, ", "), os.ErrNotExist)
	}

	cfg := new(Config)
	if err := ko.UnmarshalWithConf("", cfg, koanfUnmarshalConf); err != nil {
		return nil, fmt.Errorf("unmarshaling configuration: %w", err)
	}

	if err := defaults.Set(cfg); err != nil {
		return nil, fmt.Errorf("setting defaults: %w", err)
	}

	if buildInfo != "" {
		if err := commoncfg.UpdateConfigVersion(
			&cfg.BaseConfig,
			buildInfo,
		); err != nil {
			return nil, fmt.Errorf("updating the version configuration: %w", err)
		}
	}

	setKoanf(reflect.ValueOf(cfg), ko)

	return cfg, nil
}

var koanfSetterType = reflect.TypeFor[koanfSetter]()

func setKoanf(v reflect.Value, ko *koanf.Koanf) {
	if v.Type().Implements(koanfSetterType) {
		//nolint:forcetypeassert // Checked above
		v.Interface().(koanfSetter).setKoanf(ko)
	}

	elem := reflect.Indirect(v)
	if elem.Kind() == reflect.Struct {
		for field, val := range elem.Fields() {
			name, _, _ := strings.Cut(field.Tag.Get(koanfUnmarshalConf.Tag), ",")
			if name == "" {
				runes := []rune(field.Name)
				runes[0] = unicode.ToLower(runes[0])
				name = string(runes)
			}

			if val.Kind() != reflect.Pointer {
				val = val.Addr()
			}

			setKoanf(val, ko.Cut(name))
		}
	}
}
