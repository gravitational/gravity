package keyval

import (
	"github.com/gravitational/gravity/lib/storage"

	"github.com/gravitational/trace"
)

// CompareAndSwapSiteState swaps site state to new version only if
// it's set to the required state
func (b *backend) CompareAndSwapSiteState(domain string, old, new string) error {
	site, err := b.GetSite(domain)
	if err != nil {
		return trace.Wrap(err)
	}
	newSite := *site
	newSite.State = new
	site.State = old
	var out storage.Site
	return b.compareAndSwap(b.key(sitesP, domain, valP), newSite, site, &out, 0)
}

// GetLocalSite returns local site for a given account ID
func (b *backend) GetLocalSite(accountID string) (*storage.Site, error) {
	sites, err := b.GetSites(accountID)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, site := range sites {
		if site.Local {
			return &site, nil
		}
	}
	return nil, trace.NotFound("no local cluster found for account %v", accountID)
}

func (b *backend) CreateSite(s storage.Site) (*storage.Site, error) {
	err := s.Check()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = b.createVal(b.key(sitesP, s.Domain, valP), s, forever)
	if err != nil {
		if trace.IsAlreadyExists(err) {
			return nil, trace.Wrap(err, "cluster %v already exists", s.Domain)
		}
		return nil, trace.Wrap(err)
	}
	return &s, nil
}

func (b *backend) DeleteSite(domain string) error {
	err := b.deleteDir(b.key(sitesP, domain))
	if err != nil {
		if trace.IsNotFound(err) {
			return trace.Wrap(err, "cluster %v not found", domain)
		}
		return trace.Wrap(err)
	}
	return nil
}

func (b *backend) GetSites(accountID string) ([]storage.Site, error) {
	domains, err := b.getKeys(b.key(sitesP))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var out []storage.Site
	for _, domain := range domains {
		site, err := b.GetSite(domain)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		if site.AccountID != accountID {
			continue
		}
		out = append(out, *site)
	}
	return out, nil
}

func (b *backend) GetAllSites() ([]storage.Site, error) {
	var all []storage.Site
	accounts, err := b.GetAccounts()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, account := range accounts {
		sites, err := b.GetSites(account.ID)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		all = append(all, sites...)
	}
	return all, nil
}

func (b *backend) GetSite(domain string) (*storage.Site, error) {
	if domain == "" {
		return nil, trace.BadParameter("missing parameter SiteDomain")
	}
	var s storage.Site
	if err := b.getVal(b.key(sitesP, domain, valP), &s); err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("cluster %v not found", domain)
		}
		return nil, trace.Wrap(err)
	}
	return &s, nil
}

func (b *backend) UpdateSite(s storage.Site) (*storage.Site, error) {
	if err := s.Check(); err != nil {
		return nil, trace.Wrap(err)
	}
	err := b.updateVal(b.key(sitesP, s.Domain, valP), s, forever)
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("cluster %v not found", s.Domain)
		}
		return nil, trace.Wrap(err)
	}
	return &s, nil
}

func (b *backend) GetClusterImportStatus() (bool, error) {
	var flag bool
	err := b.getVal(b.key(importP), &flag)
	if err != nil {
		if trace.IsNotFound(err) {
			return false, trace.NotFound("no import marker found")
		}
		return false, trace.Wrap(err)
	}
	return flag, nil
}

func (b *backend) SetClusterImported() error {
	err := b.createVal(b.key(importP), true, forever)
	if err != nil {
		if trace.IsAlreadyExists(err) {
			return trace.Wrap(err, "already imported")
		}
		return trace.Wrap(err)
	}
	return nil
}
