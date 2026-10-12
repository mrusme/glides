package route

import (
	"github.com/gofiber/fiber/v3"
	"xn--gckvb8fzb.com/glides/runtime"
)

type IRouteController interface {
	GetRuntime() *runtime.Runtime
	GetPath() string
	GetEnv() *Environment
}

type RouteController struct {
	Runtime *runtime.Runtime
	Router  fiber.Router
	Routes  []IRouteController
	Path    string
	Env     *Environment
}

func (rc *RouteController) GetRuntime() *runtime.Runtime {
	return rc.Runtime
}

func (rc *RouteController) GetPath() string {
	return rc.Path
}

func (rc *RouteController) GetEnv() *Environment {
	return rc.Env
}
