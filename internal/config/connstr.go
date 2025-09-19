package config

import (
	"fmt"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
)

func MakeConnStr(conf Database) (string, error) {
	host, err := commoncfg.LoadValueFromSourceRef(conf.Host)
	if err != nil {
		return "", fmt.Errorf("loading db host: %w", err)
	}

	user, err := commoncfg.LoadValueFromSourceRef(conf.User)
	if err != nil {
		return "", fmt.Errorf("loading db user: %w", err)
	}

	password, err := commoncfg.LoadValueFromSourceRef(conf.Password)
	if err != nil {
		return "", fmt.Errorf("loading db password: %w", err)
	}

	return fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s",
		host, user, string(password), conf.Name, conf.Port), nil
}
