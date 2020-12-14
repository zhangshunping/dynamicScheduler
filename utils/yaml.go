package utils

import (
	"fmt"
	"io/ioutil"
	"gopkg.in/yaml.v2"
)

func GetRuleFromYaml(rulepath string) map[string]map[string]string {
	var c Conf
	Y:=c.GetConf(rulepath)
	return Y.Rulename
}


type Conf struct {
	Rulename map[string]map[string]string `yaml:"Rulename"`
}

func (c *Conf) GetConf(rulepath string) *Conf {
	yamlFile, err := ioutil.ReadFile(rulepath)
	if err != nil {
		fmt.Println(err.Error())
	}
	err = yaml.Unmarshal(yamlFile, c)
	if err != nil {
		fmt.Println(err.Error())
	}
	return c
}
