package client

import (
	"fmt"
	"gopkg.in/AlecAivazis/survey.v1"
	"sort"

	"github.com/bishopfox/sliver/client/assets"
)

func SelectConfig() *assets.ClientConfig {
	configs := assets.GetConfigs()

	if len(configs) == 0 {
		return nil
	}

	if len(configs) == 1 {
		for _, config := range configs {
			return config
		}
	}

	answer := struct{ Config string }{}
	qs := GetPromptForConfigs(configs)
	err := survey.Ask(qs, &answer)
	if err != nil {
		fmt.Println(err.Error())
		return nil
	}

	return configs[answer.Config]
}

func GetPromptForConfigs(configs map[string]*assets.ClientConfig) []*survey.Question {
	keys := []string{}
	for k := range configs {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	return []*survey.Question{
		{
			Name: "config",
			Prompt: &survey.Select{
				Message: "Select a server:",
				Options: keys,
				Default: keys[0],
			},
		},
	}
}
