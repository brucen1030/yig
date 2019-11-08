package tidbclient

import (
	"context"
	"database/sql"

	"github.com/journeymidnight/yig/helper"
	. "github.com/journeymidnight/yig/meta/types"
)

func (t *TidbClient) PutBucketToLifeCycle(ctx context.Context, lifeCycle LifeCycle) error {
	sqltext := "insert ignore into lifecycle(bucketname,status) values (?,?);"
	_, err := t.Client.Exec(sqltext, lifeCycle.BucketName, lifeCycle.Status)
	if err != nil {
		helper.Logger.Printf(0, "[", helper.RequestIdFromContext(ctx), "]", "Failed in PutBucketToLifeCycle: %s\n", sqltext)
		return nil
	}
	return nil
}

func (t *TidbClient) RemoveBucketFromLifeCycle(ctx context.Context, bucket *Bucket) error {
	sqltext := "delete from lifecycle where bucketname=?;"
	_, err := t.Client.Exec(sqltext, bucket.Name)
	if err != nil {
		helper.Logger.Printf(0, "[", helper.RequestIdFromContext(ctx), "]", "Failed in RemoveBucketFromLifeCycle: %s\n", sqltext)
		return nil
	}
	return nil
}

func (t *TidbClient) ScanLifeCycle(ctx context.Context, limit int, marker string) (result ScanLifeCycleResult, err error) {
	result.Truncated = false
	sqltext := "select * from lifecycle where bucketname > ? limit ?;"
	rows, err := t.Client.Query(sqltext, marker, limit)
	if err == sql.ErrNoRows {
		helper.Logger.Printf(0, "Failed in sql.ErrNoRows: %s\n", sqltext)
		err = nil
		return
	} else if err != nil {
		return
	}
	defer rows.Close()
	result.Lcs = make([]LifeCycle, 0, limit)
	var lc LifeCycle
	for rows.Next() {
		err = rows.Scan(
			&lc.BucketName,
			&lc.Status)
		if err != nil {
			helper.Logger.Printf(0, "[", helper.RequestIdFromContext(ctx), "]", "Failed in ScanLifeCycle: %s ... %s\n", result.Lcs, result.NextMarker)
			return
		}
		result.Lcs = append(result.Lcs, lc)
	}
	result.NextMarker = lc.BucketName
	if len(result.Lcs) == limit {
		result.Truncated = true
	}
	return result, nil
}

func (t *TidbClient) ScanHiddenBuckets(ctx context.Context, limit int, marker string) (buckets []string, truncated bool, err error) {
	err = nil
	truncated = false
	buckets = nil

	sqltext := "select bucketname from users where bucketname like ? and bucketname > ? order by bucketname limit ?;"
	rows, err := t.Client.Query(sqltext, HIDDEN_BUCKET_PREFIX+"%", marker, limit)
	if err == sql.ErrNoRows {
		return
	} else if err != nil {
		helper.Logger.Printf(5, "[ %s ]", "Failed in ScanHiddenBuckets: err %v patten %s marker %s limit %d sql %s", 
							helper.RequestIdFromContext(ctx), err, "'" + HIDDEN_BUCKET_PREFIX+"%" + "'", marker, limit, sqltext)
		return
	}

	defer rows.Close()

	buckets = make([]string, 0, limit)
	for rows.Next() {
		var bucketName string
		if err = rows.Scan(&bucketName); err != nil {
			return
		}

		buckets = append(buckets, bucketName)
	}

	if len(buckets) == limit {
		truncated = true
	}

	return
}
