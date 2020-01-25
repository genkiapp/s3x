package s3x

import (
	"context"
	"sort"
	"strings"

	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/query"
)

/* Design Notes
---------------

Internal functions should never claim or release locks.
Any claiming or releasing of locks should be done in the public setter+getter functions.
The reason for this is so that we can enable easy reuse of internal code.
*/

/////////////////////
// SETTER FUNCTINS //
/////////////////////

// AbortMultipartUpload is used to abort a multipart upload
func (ls *ledgerStore) AbortMultipartUpload(bucket, multipartID string) error {
	ex, err := ls.bucketExists(bucket)
	if err != nil {
		return err
	}
	if !ex {
		return ErrLedgerBucketDoesNotExist
	}
	if err := ls.l.multipartExists(multipartID); err != nil {
		return err
	}
	return ls.l.deleteMultipartID(bucket, multipartID)
}

// NewMultipartUpload is used to store the initial start of a multipart upload request
func (ls *ledgerStore) NewMultipartUpload(multipartID string, info *ObjectInfo) error {
	bucket := info.GetBucket()
	defer ls.locker.write(bucket)
	err := ls.assertBucketExits(bucket)
	if err != nil {
		return err
	}
	if ls.l.MultipartUploads == nil {
		ls.l.MultipartUploads = make(map[string]*MultipartUpload)
	}
	ls.l.MultipartUploads[multipartID] = &MultipartUpload{
		ObjectInfo: info,
		Id:         multipartID,
	}
	return nil //todo: save to ipfs
}

// PutObjectPart is used to record an individual object part within a multipart upload
func (ls *ledgerStore) PutObjectPart(bucketName, objectName, partHash, multipartID string, partNumber int64) error {
	err := ls.assertBucketExits(bucketName)
	if err != nil {
		return err
	}
	mpart, ok := ls.l.MultipartUploads[multipartID]
	if !ok {
		return ErrInvalidUploadID
	}
	mpart.ObjectParts[partHash] = ObjectPartInfo{
		Number:   partNumber,
		DataHash: partHash,
	}
	return nil //todo: save to ipfs
}

// Close shuts down the ledger datastore
func (ls *ledgerStore) Close() error {
	//todo: clean up caches
	return ls.ds.Close()
}

/////////////////////
// GETTER FUNCTINS //
/////////////////////

// GetObjectParts is used to return multipart upload parts
func (ls *ledgerStore) GetObjectParts(id string) (map[string]ObjectPartInfo, error) {
	if err := ls.l.multipartExists(id); err != nil {
		return nil, err
	}
	return ls.l.GetMultipartUploads()[id].ObjectParts, nil
}

// MultipartIDExists is used to lookup if the given multipart id exists
func (ls *ledgerStore) MultipartIDExists(id string) error {
	return ls.l.multipartExists(id)
}

// GetObjectInfos returns a list of ordered ObjectInfos with given prefix ordered by name
func (ls *ledgerStore) GetObjectInfos(ctx context.Context, bucket, prefix, startsFrom string, max int) ([]ObjectInfo, error) {
	defer ls.locker.read(bucket)()
	b, err := ls.getBucketLoaded(ctx, bucket)
	if err != nil {
		return nil, err
	}
	var names []string
	objs := b.GetBucket().GetObjects()
	for name := range objs {
		if strings.HasPrefix(name, prefix) && strings.Compare(startsFrom, name) >= 0 {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	if max > 0 && len(names) > max {
		names = names[:max]
	}
	list := make([]ObjectInfo, 0, len(names))
	for _, name := range names {
		obj, err := ls.object(ctx, bucket, name)
		if err != nil {
			return nil, err
		}
		list = append(list, obj.GetObjectInfo())
	}
	return list, nil
}

// GetObjectHash is used to retrieve the corresponding IPFS CID for an object
func (ls *ledgerStore) GetObjectHash(ctx context.Context, bucket, object string) (string, error) {
	objs, unlock, err := ls.GetObjectHashes(ctx, bucket)
	if err != nil {
		return "", err
	}
	defer unlock()
	h, ok := objs[object]
	if !ok {
		return "", ErrLedgerObjectDoesNotExist
	}
	return h, nil
}

// GetObjectHashes gets a map of object names to object hashes for all objects in a bucket.
// The returned function must be called to release a read lock, iff an error is not returned.
func (ls *ledgerStore) GetObjectHashes(ctx context.Context, bucket string) (map[string]string, func(), error) {
	unlock := ls.locker.read(bucket)
	b, err := ls.getBucketLoaded(ctx, bucket)
	if err != nil {
		unlock()
		return nil, nil, err
	}
	return b.Bucket.Objects, unlock, nil
}

// GetMultipartHashes returns the hashes of all multipart upload object parts
/* not used for now
func (ls *ledgerStore) GetMultipartHashes(bucket, multipartID string) ([]string, error) {
	ex, err := ls.bucketExists(bucket)
	if err != nil {
		return nil, err
	}
	if !ex {
		return nil, ErrLedgerBucketDoesNotExist
	}
	if err := ls.l.multipartExists(multipartID); err != nil {
		return nil, err
	}
	mpart := ls.l.MultipartUploads[bucket]
	var hashes = make([]string, len(mpart.ObjectParts))
	for i, objpart := range mpart.ObjectParts {
		hashes[i] = objpart.GetDataHash()
	}
	return hashes, nil
}*/

// GetBucketNames is used to get a slice of all bucket names our ledger currently tracks
func (ls *ledgerStore) GetBucketNames() ([]string, error) {
	//this only reads from the datastore, which have it's own synchronization, so no locking is needed.
	rs, err := ls.ds.Query(query.Query{
		Prefix:   dsBucketKey.String(),
		KeysOnly: true,
	})
	if err != nil {
		return nil, err
	}
	names := []string{}
	for r := range rs.Next() {
		names = append(names, datastore.NewKey(r.Key).BaseNamespace())
	}
	return names, nil
}

///////////////////////
// INTERNAL FUNCTINS //
///////////////////////

// multipartExists is a helper function to check if a multipart id exists in our ledger
// todo: document id
func (m *Ledger) multipartExists(id string) error {
	if m.MultipartUploads == nil {
		return ErrInvalidUploadID
	}
	if m.MultipartUploads[id].Id == "" {
		return ErrInvalidUploadID
	}
	return nil
}

func (m *Ledger) deleteMultipartID(bucketName, multipartID string) error {
	delete(m.MultipartUploads, multipartID)
	//todo: save to ipfs
	return nil
}
