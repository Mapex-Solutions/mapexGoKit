package container

import "go.uber.org/dig"

// In is an alias for dig.In, used for dependency injection with named parameters
//
// Usage example:
//
//	c.Provide(func(params struct {
//	    container.In
//	    RC *redisModel.RedisClient `name:"app"`
//	}) common.AppCache {
//	    return params.RC
//	})
type In = dig.In

// Out is an alias for dig.Out, used for providing multiple values from a single constructor
//
// Usage example:
//
//	c.Provide(func() (struct {
//	    container.Out
//	    Cache1 Cache `name:"app"`
//	    Cache2 Cache `name:"shared"`
//	}) {
//	    // return multiple providers
//	})
type Out = dig.Out

// Name is an alias for dig.Name, used to provide named dependencies
//
// Usage example:
//
//	c.Provide(func() *RedisClient {
//	    return redisClient
//	}, container.Name("app"))
var Name = dig.Name

// Group is an alias for dig.Group, used to provide grouped dependencies
//
// Usage example:
//
//	c.Provide(func() *Handler {
//	    return handler
//	}, container.Group("handlers"))
var Group = dig.Group
