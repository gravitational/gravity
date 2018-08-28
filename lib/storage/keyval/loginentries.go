package keyval

import (
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
)

func (b *backend) SetCurrentOpsCenter(o string) error {
	return trace.BadParameter("not implemented")
}

func (b *backend) GetCurrentOpsCenter() string {
	return ""
}

func (b *backend) UpsertLoginEntry(le storage.LoginEntry) (*storage.LoginEntry, error) {
	if err := le.Check(); err != nil {
		return nil, trace.Wrap(err)
	}
	if le.Created.IsZero() {
		le.Created = b.Clock.Now().UTC()
	}
	err := b.upsertVal(b.key(loginsP, le.OpsCenterURL), le, forever)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &le, nil
}

func (b *backend) DeleteLoginEntry(opsCenterURL string) error {
	err := b.deleteKey(b.key(loginsP, opsCenterURL))
	if err != nil {
		if trace.IsNotFound(err) {
			return trace.NotFound("login(%v) not found", opsCenterURL)
		}
		return trace.Wrap(err)
	}
	return trace.Wrap(err)
}

func (b *backend) GetLoginEntries() ([]storage.LoginEntry, error) {
	keys, err := b.getKeys(b.key(loginsP))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var out []storage.LoginEntry
	for _, opsURL := range keys {
		entry, err := b.GetLoginEntry(opsURL)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		out = append(out, *entry)
	}
	return out, nil
}

func (b *backend) GetLoginEntry(opsCenterURL string) (*storage.LoginEntry, error) {
	var e storage.LoginEntry
	err := b.getVal(b.key(loginsP, opsCenterURL), &e)
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("entry(%v) not found")
		}
		return nil, trace.Wrap(err)
	}
	utils.UTC(&e.Expires)
	return &e, nil
}
