package meta

import (
	"context"
	"fmt"
	"strings"
	"time"

	. "github.com/journeymidnight/yig/error"
	"github.com/journeymidnight/yig/helper"
	. "github.com/journeymidnight/yig/meta/types"
	"github.com/journeymidnight/yig/redis"
)

const (
	BUCKET_CACHE_PREFIX = "bucket:"
	USER_CACHE_PREFIX   = "user:"
)

// Note the usage info got from this method is possibly not accurate because we don't
// invalid cache when updating usage. For accurate usage info, use `GetUsage()`
func (m *Meta) GetBucket(ctx context.Context, bucketName string, willNeed bool) (bucket *Bucket, err error) {
	getBucket := func() (b helper.Serializable, err error) {
		b, err = m.Client.GetBucket(bucketName)
		helper.Logger.Info(ctx, "GetBucket CacheMiss. bucket:", bucketName)
		return b, err
	}
	toBucket := func(fields map[string]string) (interface{}, error) {
		b := &Bucket{}
		return b.Deserialize(fields)
	}

	b, err := m.Cache.Get(ctx, redis.BucketTable, BUCKET_CACHE_PREFIX, bucketName, getBucket, toBucket, willNeed)
	if err != nil {
		return
	}
	bucket, ok := b.(*Bucket)
	if !ok {
		helper.Logger.Error(ctx, "Cast b failed:", b)
		err = ErrInternalError
		return
	}
	return bucket, nil
}

func (m *Meta) GetBuckets() (buckets []*Bucket, err error) {
	buckets, err = m.Client.GetBuckets()
	return
}

func (m *Meta) UpdateUsage(ctx context.Context, bucketName string, size int64) error {
	tstart := time.Now()
	usage, err := m.Cache.HIncrBy(redis.BucketTable, BUCKET_CACHE_PREFIX, bucketName, FIELD_NAME_USAGE, size)
	if err != nil {
		helper.Logger.Error(ctx, fmt.Sprintf("failed to update bucket[%s] usage by %d, err: %v",
			bucketName, size, err))
		return err
	}
	tinc := time.Now()
	dur := tinc.Sub(tstart)
	if dur/1000000 >= 100 {
		helper.Logger.Warn(ctx, fmt.Sprintf("slow log: RedisIncrBy: bucket: %s, size: %d, takes: %d",
			bucketName, size, dur))
	}

	err = m.addBucketUsageSyncEvent(bucketName)
	if err != nil {
		helper.Logger.Error(ctx, fmt.Sprintf("failed to add bucket usage sync event for bucket: %s, err: %v",
			bucketName, err))
		return err
	}
	helper.Logger.Info(ctx, "incr usage for bucket: ", bucketName, ", updated to ", usage)
	tend := time.Now()
	dur = tend.Sub(tinc)
	if dur/1000000 >= 100 {
		helper.Logger.Error(ctx, fmt.Sprintf("slow log: AddBucketUsageSyncEvent: bucket: %s, size: %d, takes: %d",
			bucketName, size, dur))
	}
	dur = tend.Sub(tstart)
	if dur/1000000 >= 100 {
		helper.Logger.Error(ctx, fmt.Sprintf("slow log: cache update, bucket: %s, size: %d, takes: %d",
			bucketName, size, dur))
	}
	return nil
}

func (m *Meta) GetUsage(ctx context.Context, bucketName string) (int64, error) {
	usage, err := m.Cache.HGetInt64(redis.BucketTable, BUCKET_CACHE_PREFIX, bucketName, FIELD_NAME_USAGE)
	if err != nil {
		helper.Logger.Info(ctx, "failed to get usage for bucket: ", bucketName, ", err: ", err)
		return 0, err
	}
	return usage, nil
}

func (m *Meta) GetBucketInfo(ctx context.Context, bucketName string) (*Bucket, error) {
	m.Cache.Remove(redis.BucketTable, BUCKET_CACHE_PREFIX, bucketName)
	bucket, err := m.GetBucket(ctx, bucketName, true)
	if err != nil {
		return bucket, err
	}
	return bucket, nil
}

func (m *Meta) GetUserInfo(ctx context.Context, uid string) ([]string, error) {
	m.Cache.Remove(redis.UserTable, USER_CACHE_PREFIX, uid)
	buckets, err := m.GetUserBuckets(ctx, uid, true)
	if err != nil {
		return nil, err
	}
	return buckets, nil
}

/*
* init bucket usage cache when meta is newed.
*
 */
func (m *Meta) InitBucketUsageCache() error {
	// the map contains the bucket usage which are not in cache.
	bucketUsageMap := make(map[string]*Bucket)
	// the map contains the bucket usage which are in cache and will be synced into database.
	bucketUsageCacheMap := make(map[string]int64)
	// the usage in buckets table is accurate now.
	buckets, err := m.Client.GetBuckets()
	if err != nil {
		helper.Logger.Error(nil, fmt.Sprintf("failed to get buckets from db. err: ", err))
		return err
	}

	// init the bucket usage key in cache.
	for _, bucket := range buckets {
		bucketUsageMap[bucket.Name] = bucket
	}

	// try to get all bucket usage keys from cache.
	pattern := fmt.Sprintf("%s*", BUCKET_CACHE_PREFIX)
	bucketsInCache, err := m.Cache.Keys(redis.BucketTable, pattern)
	if err != nil {
		helper.Logger.Error(nil, fmt.Sprintf("failed to get bucket usage from cache, err: ", err))
		return err
	}

	if len(bucketsInCache) > 0 {
		// query all usages from cache.
		for _, bic := range bucketsInCache {
			elems := strings.Split(bic, ":")
			name := bic
			if len(elems) > 0 {
				name = elems[1]
			}
			usage, err := m.Cache.HGetInt64(redis.BucketTable, BUCKET_CACHE_PREFIX, name, FIELD_NAME_USAGE)
			if err != nil {
				helper.Logger.Error(nil, fmt.Sprintf("failed to get usage for bucket: ", name, " with err: ", err))
				continue
			}
			// add the to be synced usage.
			bucketUsageCacheMap[name] = usage
			if _, ok := bucketUsageMap[name]; ok {
				// if the key already exists in cache, then delete it from map
				delete(bucketUsageMap, name)
			}
		}

	}

	// init the bucket usage in cache.
	if len(bucketUsageMap) > 0 {
		for _, bk := range bucketUsageMap {
			fields, err := bk.Serialize()
			if err != nil {
				helper.Logger.Error(nil, fmt.Sprintf("failed to serialize for bucket: ", bk.Name, " with err: ", err))
				return err
			}
			_, err = m.Cache.HMSet(redis.BucketTable, BUCKET_CACHE_PREFIX, bk.Name, fields)
			if err != nil {
				helper.Logger.Error(nil, fmt.Sprintf("failed to set bucket to cache: ", bk.Name, " with err: ", err))
				return err
			}
		}

	}
	// sync the buckets usage in cache into database.
	if len(bucketUsageCacheMap) > 0 {
		err = m.Client.UpdateUsages(bucketUsageCacheMap, nil)
		if err != nil {
			helper.Logger.Error(nil, fmt.Sprintf("failed to sync usages to database, err: ", err))
			return err
		}
	}
	return nil
}

func (m *Meta) bucketUsageSync() error {
	buckets, err := m.Cache.HGetAll(redis.BucketTable, SYNC_EVENT_BUCKET_USAGE_PREFIX, "trigger")
	if err != nil {
		helper.Logger.Error(nil, fmt.Sprintf("failed to get buckets whose usage are changed, err: %v", err))
		return err
	}
	if len(buckets) <= 0 {
		return nil
	}
	var cacheRemove []string
	for k, _ := range buckets {
		usage, err := m.Cache.HGetInt64(redis.BucketTable, BUCKET_CACHE_PREFIX, k, FIELD_NAME_USAGE)
		if err != nil {
			helper.Logger.Error(nil, fmt.Sprintf("failed to get usage for bucket: %s, err: %v", k, err))
			continue
		}
		err = m.Client.UpdateUsage(k, usage, nil)
		if err != nil {
			helper.Logger.Error(nil, "failed to update bucket usage ", usage, " to bucket: ", k, " err: ", err)
			continue
		}
		cacheRemove = append(cacheRemove, k)

		helper.Logger.Info(nil, "succeed to update bucket usage ", usage, " for bucket: ", k)
	}
	// remove the bucket usage sync event.
	if len(cacheRemove) > 0 {
		_, err = m.Cache.HDel(redis.BucketTable, SYNC_EVENT_BUCKET_USAGE_PREFIX, "trigger", cacheRemove)
		if err != nil {
			helper.Logger.Error(nil, fmt.Sprintf("failed to unset the bucket usage change event for %v, err: %v", cacheRemove, err))
			return err
		}
		helper.Logger.Info(nil, fmt.Sprintf("succeed to remove bucket usage trigger for %v", cacheRemove))
	}
	return nil
}

func (m *Meta) addBucketUsageSyncEvent(bucketName string) error {
	_, err := m.Cache.HSet(redis.BucketTable, SYNC_EVENT_BUCKET_USAGE_PREFIX, "trigger", bucketName, 1)
	if err != nil {
		return err
	}
	return nil
}
