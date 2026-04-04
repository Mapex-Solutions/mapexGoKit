package container

import (
	"sync"

	"go.uber.org/dig"
)

var (
	instance *dig.Container
	once     sync.Once
)

// InitContainer inicializa o contêiner singleton
func InitContainer() *dig.Container {
	once.Do(func() {
		instance = dig.New()
	})

	return instance
}
