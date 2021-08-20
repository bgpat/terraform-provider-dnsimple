package dnsimple

import (
	"context"
	"fmt"
	"sync"

	"github.com/dnsimple/dnsimple-go/dnsimple"
	"golang.org/x/sync/singleflight"
)

var perPage = 100

type cachedRecordsService struct {
	client *dnsimple.Client
	cache  sync.Map
	group  singleflight.Group
}

type zoneCacheKey struct {
	accountID string
	zoneName  string
}

func newCachedRecordsService(client *dnsimple.Client) *cachedRecordsService {
	return &cachedRecordsService{client: client}
}

// GetRecord returns a dnsimple record.
func (s *cachedRecordsService) GetRecord(ctx context.Context, accountID, zoneName string, recordID int64) (*dnsimple.ZoneRecord, error) {
	key := zoneCacheKey{accountID: accountID, zoneName: zoneName}
	records, err, _ := s.group.Do(key.String(), func() (interface{}, error) {
		return s.getAllRecords(ctx, key)
	})
	if err != nil {
		return nil, err
	}
	for _, record := range records.([]dnsimple.ZoneRecord) {
		if record.ID == recordID {
			return &record, nil
		}
	}
	return nil, fmt.Errorf("404")
}

// CreateRecord is a wrapper function for dnsimple.ZonesServices.CreateRecord.
func (s *cachedRecordsService) CreateRecord(ctx context.Context, accountID, zoneName string, recordAttributes dnsimple.ZoneRecordAttributes) (*dnsimple.ZoneRecordResponse, error) {
	s.clearRecordsCache(zoneCacheKey{accountID: accountID, zoneName: zoneName})
	return s.client.Zones.CreateRecord(ctx, accountID, zoneName, recordAttributes)
}

// UpdateRecord is a wrapper function for dnsimple.ZonesServices.UpdateRecord.
func (s *cachedRecordsService) UpdateRecord(ctx context.Context, accountID, zoneName string, recordID int64, recordAttributes dnsimple.ZoneRecordAttributes) (*dnsimple.ZoneRecordResponse, error) {
	s.clearRecordsCache(zoneCacheKey{accountID: accountID, zoneName: zoneName})
	return s.client.Zones.UpdateRecord(ctx, accountID, zoneName, recordID, recordAttributes)
}

// DeleteRecord is a wrapper function for dnsimple.ZonesServices.DeleteRecord.
func (s *cachedRecordsService) DeleteRecord(ctx context.Context, accountID, zoneName string, recordID int64) (*dnsimple.ZoneRecordResponse, error) {
	s.clearRecordsCache(zoneCacheKey{accountID: accountID, zoneName: zoneName})
	return s.client.Zones.DeleteRecord(ctx, accountID, zoneName, recordID)
}

func (s *cachedRecordsService) getAllRecords(ctx context.Context, key zoneCacheKey) ([]dnsimple.ZoneRecord, error) {
	if v, ok := s.cache.Load(key); ok {
		return v.([]dnsimple.ZoneRecord), nil
	}

	var records []dnsimple.ZoneRecord
	for page := 1; ; page++ {
		options := &dnsimple.ZoneRecordListOptions{
			ListOptions: dnsimple.ListOptions{
				Page:    &page,
				PerPage: &perPage,
			},
		}
		resp, err := s.client.Zones.ListRecords(ctx, key.accountID, key.zoneName, options)
		if err != nil {
			return nil, err
		}
		if records == nil {
			records = make([]dnsimple.ZoneRecord, 0, resp.Pagination.TotalEntries)
		}
		records = append(records, resp.Data...)
		if resp.Pagination.CurrentPage >= resp.Pagination.TotalPages {
			break
		}
	}
	s.cache.Store(key, records)
	return records, nil
}
func (s *cachedRecordsService) clearRecordsCache(key zoneCacheKey) {
	s.group.Forget(key.String())
	s.cache.Delete(key)
}

func (k *zoneCacheKey) String() string {
	return fmt.Sprintf("%#v", k)
}
