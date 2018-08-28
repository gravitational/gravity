package keyval

import (
	"github.com/gravitational/gravity/lib/storage"

	"github.com/gravitational/trace"
	"github.com/pborman/uuid"
)

func (b *backend) CreateProgressEntry(p storage.ProgressEntry) (*storage.ProgressEntry, error) {
	err := p.Check()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if p.ID == "" {
		p.ID = uuid.New()
	}
	if _, err := b.GetSite(p.SiteDomain); err != nil {
		return nil, trace.Wrap(err)
	}
	err = b.createVal(b.key(sitesP, p.SiteDomain, operationsP, p.OperationID, progressP, p.ID), p, forever)
	if err != nil {
		if trace.IsAlreadyExists(err) {
			return nil, trace.Wrap(err, "progress(%v) already exists", p.ID)
		}
		return nil, trace.Wrap(err)
	}
	return &p, nil
}

func (b *backend) GetLastProgressEntry(siteDomain, operationID string) (*storage.ProgressEntry, error) {
	if siteDomain == "" {
		return nil, trace.BadParameter("missing site domain")
	}
	if operationID == "" {
		return nil, trace.BadParameter("missing operation id")
	}
	ids, err := b.getKeys(b.key(sitesP, siteDomain, operationsP, operationID, progressP))
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("no progress entries for %v %v found", siteDomain, operationID)
		}
		return nil, trace.Wrap(err)
	}
	var p *storage.ProgressEntry
	if len(ids) == 0 {
		return nil, trace.NotFound("no progress entries for %v %v found", siteDomain, operationID)
	}
	for _, id := range ids {
		var e storage.ProgressEntry
		err := b.getVal(b.key(sitesP, siteDomain, operationsP, operationID, progressP, id), &e)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if p == nil || e.Created.After(p.Created) {
			p = &e
		}
	}
	return p, nil
}

func (b *backend) CreateAppProgressEntry(p storage.AppProgressEntry) (*storage.AppProgressEntry, error) {
	err := p.Check()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if p.ID == "" {
		p.ID = uuid.New()
	}
	key := b.key(appOperationsP, p.OperationID, progressP, p.ID)
	err = b.createVal(key, p, forever)
	if err != nil {
		if trace.IsAlreadyExists(err) {
			return nil, trace.Wrap(err, "progress(%v) already exists", p.ID)
		}
		return nil, trace.Wrap(err)
	}
	return &p, nil
}

func (b *backend) GetLastAppProgressEntry(operationID string) (*storage.AppProgressEntry, error) {
	key := b.key(appOperationsP, operationID, progressP)

	ids, err := b.getKeys(key)
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("no progress entries for %v found", operationID)
		}
		return nil, trace.Wrap(err)
	}
	var p *storage.AppProgressEntry
	if len(ids) == 0 {
		return nil, trace.NotFound("no progress entries for %v found", operationID)
	}
	for _, id := range ids {
		var e storage.AppProgressEntry
		entryKey := b.key(appOperationsP, operationID,
			progressP, id)
		err := b.getVal(entryKey, &e)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if p == nil || e.Created.After(p.Created) {
			p = &e
		}
	}
	return p, nil
}
