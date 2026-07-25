package runtime

import (
	"embed"
)

func (rt *Runtime) AddEmbed(id string, ed *embed.FS) (err error) {
	rt.embeds[id] = ed
	return nil
}

func (rt *Runtime) GetEmbed(id string) (ed *embed.FS) {
	var exists bool = false

	if ed, exists = rt.embeds[id]; exists == false {
		return nil
	}

	return ed
}

func (rt *Runtime) GetEmbedOk(id string) (ed *embed.FS, ok bool) {
	var exists bool = false

	if ed, exists = rt.embeds[id]; exists == false {
		return nil, false
	}

	return ed, true
}
