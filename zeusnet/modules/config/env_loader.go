package config

import (
	"os"
	"reflect"
)

type EnvLoader struct {
	EnableFrr     string
	ContainerName string
}

// LoadEnv 进行环境变量的加载
func (envLoader *EnvLoader) LoadEnv() {
	envLoader.EnableFrr = os.Getenv("ENABLE_FRR")
	envLoader.ContainerName = os.Getenv("CONTAINER_NAME")
}

// IsAllVariablesNone 判断是否所有的都为空
func (envLoader *EnvLoader) IsAllVariablesNone() bool {
	v := reflect.ValueOf(envLoader).Elem()
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		field := v.Field(i)
		if field.Kind() == reflect.String {
			value := field.String()
			if value != "" {
				return false
			}
		}
	}
	return true
}
