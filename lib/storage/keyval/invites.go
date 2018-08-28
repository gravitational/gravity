package keyval

import (
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
)

func (b *backend) UpsertUserInvite(invite storage.UserInvite) (*storage.UserInvite, error) {
	if err := invite.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	_, err := b.GetUser(invite.Name)
	if err == nil {
		return nil, trace.BadParameter("user(%v) already registered", invite.Name)
	}

	err = b.upsertVal(b.key(invitesP, invite.Name), invite, invite.ExpiresIn)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &invite, nil
}

func (b *backend) DeleteUserInvite(name string) error {
	err := b.deleteKey(b.key(invitesP, name))
	if err != nil {
		if trace.IsNotFound(err) {
			return trace.NotFound("invite(%v) not found", name)
		}
	}
	return trace.Wrap(err)
}

func (b *backend) GetUserInvites() ([]storage.UserInvite, error) {
	emails, err := b.getKeys(b.key(invitesP))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var out []storage.UserInvite
	for _, email := range emails {
		i, err := b.GetUserInvite(email)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		out = append(out, *i)
	}
	return out, nil
}

func (b *backend) GetUserInvite(email string) (*storage.UserInvite, error) {
	var invite storage.UserInvite
	err := b.getVal(b.key(invitesP, email), &invite)
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("invite(%v) not found", email)
		}
	}

	utils.UTC(&invite.Created)
	return &invite, nil
}
