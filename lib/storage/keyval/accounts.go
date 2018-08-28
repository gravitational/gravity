package keyval

import (
	"github.com/gravitational/gravity/lib/storage"

	"github.com/gravitational/trace"
	"github.com/pborman/uuid"
)

func (b *backend) CreateAccount(a storage.Account) (*storage.Account, error) {
	if a.ID == "" {
		a.ID = uuid.New()
	}
	if err := b.createVal(b.key(accountsP, a.ID, valP), a, forever); err != nil {
		if trace.IsAlreadyExists(err) {
			return nil, trace.Wrap(err, "account(org=%v) already exists", a.Org)
		}
		return nil, trace.Wrap(err)
	}
	return &a, nil
}

func (b *backend) DeleteAccount(id string) error {
	// we need to delete all users associated with this account
	// as they are in a different hierarchy from the /accounts dir
	users, err := b.getUsers()
	if err != nil {
		return trace.Wrap(err)
	}
	for _, user := range users {
		if user.GetAccountID() == id {
			err := b.DeleteUser(user.GetName())
			if err != nil {
				if trace.IsNotFound(err) {
					continue
				}
				return trace.Wrap(err)
			}
		}
	}
	// sites
	sites, err := b.GetSites(id)
	if err != nil {
		return trace.Wrap(err)
	}
	for _, site := range sites {
		if site.AccountID == id {
			err := b.DeleteSite(site.Domain)
			if err != nil {
				if trace.IsNotFound(err) {
					continue
				}
				return trace.Wrap(err)
			}
		}
	}
	err = b.deleteDir(b.key(accountsP, id))
	return trace.Wrap(err)
}

func (b *backend) GetAccounts() ([]storage.Account, error) {
	var out []storage.Account
	keys, err := b.getKeys(b.key(accountsP))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, id := range keys {
		a, err := b.GetAccount(id)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		out = append(out, *a)
	}
	return out, nil
}

func (b *backend) GetAccount(id string) (*storage.Account, error) {
	var a storage.Account
	if err := b.getVal(b.key(accountsP, id, valP), &a); err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.Wrap(err, "account(id=%v) not found", id)
		}
		return nil, trace.Wrap(err)
	}
	return &a, nil
}
