package config

import "os"

type EnvLoader struct {
	EnableFrr     string
	ContainerName string
}

func (envLoader *EnvLoader) LoadEnv() {
	envLoader.EnableFrr = os.Getenv("ENABLE_FRR")
	envLoader.ContainerName = os.Getenv("CONTAINER_NAME")
}
