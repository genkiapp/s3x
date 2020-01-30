package s3x

import (
	"context"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"time"

	minio "github.com/RTradeLtd/s3x/cmd"
)

// ListObjects lists all blobs in S3 bucket filtered by prefix
func (x *xObjects) ListObjects(
	ctx context.Context,
	bucket, prefix, marker, delimiter string,
	maxKeys int,
) (loi minio.ListObjectsInfo, e error) {
	// TODO(bonedaddy): implement complex search (George: prefix implemented)
	objs, err := x.ledgerStore.GetObjectInfos(ctx, bucket, prefix, "", 0)
	if err != nil {
		return loi, x.toMinioErr(err, bucket, "", "")
	}
	loi.Objects = make([]minio.ObjectInfo, 0, len(objs))
	for _, obj := range objs {
		loi.Objects = append(loi.Objects, x.getMinioObjectInfo(&obj))
	}
	// TODO(bonedaddy): consider if we should use the following helper func
	// return minio.FromMinioClientListBucketResult(bucket, result), nil
	return loi, nil
}

// ListObjectsV2 lists all objects in B2 bucket filtered by prefix, returns upto max 1000 entries at a time.
func (x *xObjects) ListObjectsV2(
	ctx context.Context,
	bucket, prefix, continuationToken, delimiter string,
	maxKeys int,
	fetchOwner bool,
	startAfter string,
) (loi minio.ListObjectsV2Info, err error) {
	objs, err := x.ledgerStore.GetObjectInfos(ctx, bucket, prefix, startAfter, 1000)
	if err != nil {
		return loi, x.toMinioErr(err, bucket, "", "")
	}
	loi.Objects = make([]minio.ObjectInfo, 0, len(objs))
	for _, obj := range objs {
		loi.Objects = append(loi.Objects, x.getMinioObjectInfo(&obj))
	}
	return loi, nil
}

// GetObjectNInfo - returns object info and locked object ReadCloser
func (x *xObjects) GetObjectNInfo(
	ctx context.Context,
	bucket, object string,
	rs *minio.HTTPRangeSpec,
	h http.Header,
	lockType minio.LockType,
	opts minio.ObjectOptions,
) (gr *minio.GetObjectReader, err error) {
	objinfo, err := x.GetObjectInfo(ctx, bucket, object, opts)
	if err != nil {
		return gr, err // the error from this is already properly converted
	}
	var startOffset, length int64
	startOffset, length, err = rs.GetOffsetLength(objinfo.Size)
	if err != nil {
		return nil, err
	}
	pr, pw := io.Pipe()
	go func() {
		err := x.GetObject(ctx, bucket, object, startOffset, length, pw, objinfo.ETag, opts)
		_ = pw.CloseWithError(err)
	}()
	// Setup cleanup function to cause the above go-routine to
	// exit in case of partial read
	pipeCloser := func() { pr.Close() }
	return minio.NewGetObjectReaderFromReader(pr, objinfo, opts.CheckCopyPrecondFn, pipeCloser)
}

// GetObject reads an object from TemporalX. Supports additional
// parameters like offset and length which are synonymous with
// HTTP Range requests.
//
// startOffset indicates the starting read location of the object.
// length indicates the total length of the object.
func (x *xObjects) GetObject(
	ctx context.Context,
	bucket, object string,
	startOffset, length int64,
	writer io.Writer,
	etag string,
	opts minio.ObjectOptions,
) error {
	objData, err := x.ledgerStore.ObjectData(ctx, bucket, object)
	if err != nil {
		return x.toMinioErr(err, bucket, object, "")
	}
	end := startOffset + length
	objSize := int64(len(objData))
	if objSize < end {
		return minio.InvalidRange{
			OffsetBegin:  startOffset,
			OffsetEnd:    end,
			ResourceSize: objSize,
		}
	}
	_, err = writer.Write(objData[startOffset:end])
	return err
}

// GetObjectInfo reads object info and replies back ObjectInfo
func (x *xObjects) GetObjectInfo(
	ctx context.Context,
	bucket, object string,
	opts minio.ObjectOptions,
) (objInfo minio.ObjectInfo, err error) {
	oi, err := x.ledgerStore.ObjectInfo(ctx, bucket, object)
	return x.getMinioObjectInfo(oi), x.toMinioErr(err, bucket, object, "")
}

//newObjectInfo create an ObjectInfo
func newObjectInfo(bucket, object string, size int, opts minio.ObjectOptions) ObjectInfo {
	// TODO(bonedaddy): ensure consistency with the way s3 and b2 handle this
	obinfo := ObjectInfo{
		Bucket: bucket,
		Name:   object,
		Size_:  int64(size),
	}
	if !isTest { // creates consistent hashes for testing
		obinfo.ModTime = time.Now().UTC()
	}
	for k, v := range opts.UserDefined {
		switch strings.ToLower(k) {
		case "content-encoding":
			obinfo.ContentEncoding = v
		case "content-disposition":
			obinfo.ContentDisposition = v
		case "content-language":
			obinfo.ContentLanguage = v
		case "content-type":
			obinfo.ContentType = v
		}
	}
	return obinfo
}

// PutObject creates a new object with the incoming data,
func (x *xObjects) PutObject(
	ctx context.Context,
	bucket, object string,
	r *minio.PutObjReader,
	opts minio.ObjectOptions,
) (minio.ObjectInfo, error) {
	err := x.ledgerStore.AssertBucketExits(bucket)
	if err != nil {
		return minio.ObjectInfo{}, x.toMinioErr(err, bucket, "", "")
	}

	data, err := ioutil.ReadAll(r)
	if err != nil {
		return minio.ObjectInfo{}, x.toMinioErr(err, bucket, object, "")
	}
	obinfo := newObjectInfo(bucket, object, len(data), opts)
	dataHash, err := ipfsSaveBytes(ctx, x.dagClient, data)
	if err != nil {
		return minio.ObjectInfo{}, x.toMinioErr(err, bucket, object, "")
	}
	// add the object to bucket
	err = x.ledgerStore.PutObject(ctx, bucket, object, &Object{
		DataHash:   dataHash,
		ObjectInfo: obinfo,
	})
	if err != nil {
		return minio.ObjectInfo{}, x.toMinioErr(err, bucket, object, "")
	}
	log.Printf("bucket-name: %s, object-name: %s", bucket, object)
	// convert the proto object into a minio.ObjectInfo type
	return x.getMinioObjectInfo(&obinfo), nil
}

// CopyObject copies an object from source bucket to a destination bucket.
func (x *xObjects) CopyObject(
	ctx context.Context,
	srcBucket string,
	srcObject string,
	dstBucket string,
	dstObject string,
	srcInfo minio.ObjectInfo,
	srcOpts, dstOpts minio.ObjectOptions,
) (objInfo minio.ObjectInfo, err error) {
	// TODO(bonedaddy): implement usage of options
	// TODO(bonedaddy): ensure we properly update the ledger with the destination object
	// TODO(bonedaddy): ensure the destination object is properly adjusted with metadata

	//lock ordering by bucket name
	if srcBucket == dstBucket {
		defer x.ledgerStore.locker.write(dstBucket)()
	} else if strings.Compare(srcBucket, dstBucket) > 0 {
		defer x.ledgerStore.locker.read(srcBucket)()
		defer x.ledgerStore.locker.write(dstBucket)()
	} else {
		defer x.ledgerStore.locker.write(dstBucket)()
		defer x.ledgerStore.locker.read(srcBucket)()
	}

	// ensure destination bucket exists
	err = x.ledgerStore.assertBucketExits(dstBucket)
	if err != nil {
		return objInfo, x.toMinioErr(err, dstBucket, "", "")
	}

	obj1, err := x.ledgerStore.object(ctx, srcBucket, srcObject)
	if err != nil {
		return objInfo, x.toMinioErr(err, srcBucket, srcObject, "")
	}
	if obj1 == nil {
		return objInfo, x.toMinioErr(ErrLedgerObjectDoesNotExist, srcBucket, srcObject, "")
	}

	//copy object so the original will not be modified
	data, err := obj1.Marshal()
	if err != nil {
		panic(err)
	}
	obj := &Object{}
	if err = obj.Unmarshal(data); err != nil {
		panic(err)
	}

	// update relevant fields
	obj.ObjectInfo.Name = dstObject
	obj.ObjectInfo.Bucket = dstBucket
	obj.ObjectInfo.ModTime = time.Now().UTC()

	err = x.ledgerStore.putObject(ctx, dstBucket, dstObject, obj)
	if err != nil {
		return objInfo, x.toMinioErr(err, dstBucket, dstObject, "")
	}
	log.Printf(
		"dst-bucket: %s,  dst-object: %s\n",
		dstBucket, dstObject,
	)
	objInfo = x.getMinioObjectInfo(&obj.ObjectInfo)
	return objInfo, x.toMinioErr(err, dstBucket, dstObject, "")
}

// DeleteObject deletes a blob in bucket
func (x *xObjects) DeleteObject(
	ctx context.Context,
	bucket, object string,
) error {
	err := x.ledgerStore.RemoveObject(ctx, bucket, object)
	return x.toMinioErr(err, bucket, object, "")
}

func (x *xObjects) DeleteObjects(
	ctx context.Context,
	bucket string,
	objects []string,
) ([]error, error) {
	missing, err := x.ledgerStore.RemoveObjects(ctx, bucket, objects...)
	if err != nil {
		return nil, x.toMinioErr(err, bucket, "", "")
	}
	// TODO(bonedaddy): implement removal from ipfs
	errs := make([]error, len(missing))
	for i, m := range missing {
		errs[i] = x.toMinioErr(ErrLedgerObjectDoesNotExist, bucket, m, "")
	}
	return errs, nil
}
