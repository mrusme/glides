package runtime

import (
	"embed"
)

func (rt *Runtime) AddEmbed(id string, ed *embed.FS) {
	rt.embeds[id] = ed
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
