package container

import "go.uber.org/dig"

// GetContainer retorna a instância singleton do contêiner
func GetContainer() *dig.Container {
	return instance
}
