package route

var defaultTitle string

type Environment struct {
	Title string
}

func NewEnv() *Environment {
	env := new(Environment)

	env.Title = defaultTitle

	return env
}
