package database

import (
	"xn--gckvb8fzb.com/glides/runtime"
)

const ServiceID string = "_database"

func From(rt *runtime.Runtime) *Database {
	if srv, _ := rt.GetService(ServiceID); srv != nil {
		return srv.(*Database)
	}

	return nil
}
