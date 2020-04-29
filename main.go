package main

import (
	"bufio"
	"context"
	"encoding/hex"
	"fmt"
	"os"
	"sync"

	"cloud.google.com/go/storage"
	"google.golang.org/api/cloudresourcemanager/v1"
	"google.golang.org/api/iterator"

	"flag"

	"github.com/golang/glog"
	"gopkg.in/avro.v0"
)

var (
	outputFile      *string
	includeVersions *bool
	VERSION         string = "0.1"
)

type GcsFile struct {
	ProjectId  string
	BucketName string
	Object     storage.ObjectAttrs
}

type AvroAcl struct {
	Entity            string `avro:"entity"`
	EntityID          string `avro:"entity_id"`
	Role              string `avro:"role"`
	Domain            string `avro:"domain"`
	Email             string `avro:"email"`
	TeamProjectNumber string `avro:"team_project_number"`
	TeamProjectTeam   string `avro:"team_project_team"`
}

type AvroFile struct {
	ProjectId               string              `avro:"project_id"`
	Bucket                  string              `avro:"bucket"`
	Name                    string              `avro:"name"`
	ContentType             string              `avro:"content_type"`
	ContentLanguage         string              `avro:"content_language"`
	CacheControl            string              `avro:"cache_control"`
	EventBasedHold          bool                `avro:"event_based_hold"`
	TemporaryHold           bool                `avro:"temporary_hold"`
	RetentionExpirationTime int64               `avro:"retention_expiration_time"`
	ACL                     []map[string]string `avro:"acl"`
	PredefinedACL           string              `avro:"predefined_acl"`
	Owner                   string              `avro:"owner"`
	Size                    int64               `avro:"size"`
	ContentEncoding         string              `avro:"content_encoding"`
	ContentDisposition      string              `avro:"content_disposition"`
	MD5                     string              `avro:"md5"`
	CRC32C                  int32               `avro:"crc32c"`
	MediaLink               string              `avro:"media_link"`
	//Metadata           map[string]string `avro:"metadata"`
	Generation        int64  `avro:"generation"`
	Metageneration    int64  `avro:"metageneration"`
	StorageClass      string `avro:"storage_class"`
	Created           int64  `avro:"created"`
	Deleted           int64  `avro:"deleted"`
	Updated           int64  `avro:"updated"`
	CustomerKeySHA256 string `avro:"customer_key_sha256"`
	KMSKeyName        string `avro:"kms_key_name"`
	Etag              string `avro:"etag"`
}

func objectToAvro(ProjectId string, file storage.ObjectAttrs) (*AvroFile, error) {

	avroFile := new(AvroFile)
	avroFile.ProjectId = ProjectId
	avroFile.Bucket = file.Bucket
	avroFile.Name = file.Name
	avroFile.ContentType = file.ContentType
	avroFile.ContentLanguage = file.ContentLanguage

	avroFile.CacheControl = file.CacheControl
	avroFile.EventBasedHold = file.EventBasedHold
	avroFile.TemporaryHold = file.TemporaryHold
	avroFile.RetentionExpirationTime = file.RetentionExpirationTime.UnixNano() / 1000000

	ACLs := make([]map[string]string, 0)
	for _, acl := range file.ACL {
		_acl := make(map[string]string)
		_acl["entity"] = string(acl.Entity)
		_acl["entity_id"] = acl.EntityID
		_acl["role"] = string(acl.Role)
		_acl["domain"] = acl.Domain
		_acl["email"] = acl.Email
		if acl.ProjectTeam != nil {
			_acl["team_project_number"] = acl.ProjectTeam.ProjectNumber
			_acl["team_project_team"] = acl.ProjectTeam.Team
		}
		ACLs = append(ACLs, _acl)
	}
	avroFile.ACL = ACLs

	avroFile.PredefinedACL = file.PredefinedACL
	avroFile.Owner = file.Owner
	avroFile.Size = file.Size
	avroFile.MD5 = hex.EncodeToString(file.MD5)
	avroFile.CRC32C = int32(file.CRC32C)
	avroFile.MediaLink = file.MediaLink
	// Metadata is not returned with the query
	//avroFile.Metadata = file.Metadata
	avroFile.Generation = file.Generation
	avroFile.Metageneration = file.Metageneration
	avroFile.StorageClass = file.StorageClass
	if !file.Created.IsZero() {
		avroFile.Created = file.Created.UnixNano() / 1000
	} else {
		avroFile.Created = 0
	}
	if !file.Deleted.IsZero() {
		avroFile.Deleted = file.Deleted.UnixNano() / 1000
	} else {
		avroFile.Deleted = 0
	}
	if !file.Updated.IsZero() {
		avroFile.Updated = file.Updated.UnixNano() / 1000
	} else {
		avroFile.Updated = 0
	}
	avroFile.CustomerKeySHA256 = file.CustomerKeySHA256
	avroFile.KMSKeyName = file.KMSKeyName
	avroFile.Etag = file.Etag
	return avroFile, nil
}

func processProject(wg *sync.WaitGroup, ctx *context.Context, objectCh chan GcsFile, project cloudresourcemanager.Project) {
	defer wg.Done()

	glog.Warningf("Processing project %s...", project.ProjectId)

	client, err := storage.NewClient(*ctx)
	if err != nil {
		panic(err)
	}

	buckets := client.Buckets(*ctx, project.ProjectId)
	for bucketAttrs, err := buckets.Next(); err != iterator.Done; bucketAttrs, err = buckets.Next() {
		bucket := client.Bucket(bucketAttrs.Name)
		var query *storage.Query = nil
		if *includeVersions {
			query = &storage.Query{Versions: true}
		}
		objects := bucket.Objects(*ctx, query)
		for objectAttrs, err := objects.Next(); err != iterator.Done; objectAttrs, err = objects.Next() {
			glog.Infof("Processing file %s (bucket %s, project %s)...", objectAttrs.Name, bucketAttrs.Name, project.ProjectId)
			item := GcsFile{ProjectId: project.ProjectId, BucketName: bucketAttrs.Name, Object: *objectAttrs}
			objectCh <- item
		}
	}
}

func main() {
	os.Stderr.WriteString(fmt.Sprintf("Google Cloud Storage object metadata to BigQuery, version %s\n", VERSION))

	outputFile = flag.String("file", "gcs.avro", "output file name (default gcs.avro)")
	includeVersions = flag.Bool("versions", false, "include GCS object versions")
	flag.Parse()

	ctx := context.Background()

	crmService, err := cloudresourcemanager.NewService(ctx)
	if err != nil {
		panic(err)
	}

	glog.Warning("Retrieving a list of all projects...")
	projectsService := crmService.Projects
	projectsList := make([]cloudresourcemanager.Project, 0)
	pageToken := ""
	for {
		response, err := projectsService.List().PageToken(pageToken).Do()
		if err != nil {
			panic(err)
		}

		for _, project := range response.Projects {
			projectsList = append(projectsList, *project)
		}

		pageToken = response.NextPageToken
		if pageToken == "" {
			break
		}
	}

	var wg sync.WaitGroup

	objectCh := make(chan GcsFile)
	wg.Add(1)
	go func() {
		defer wg.Done()

		var wgProject sync.WaitGroup
		wgProject.Add(len(projectsList))
		for _, project := range projectsList {
			go processProject(&wgProject, &ctx, objectCh, project)
		}
		wgProject.Wait()
		close(objectCh)
	}()

	schema, err := avro.ParseSchemaFile("gcs2bq.avsc")
	if err != nil {
		panic(err)
	}
	writer := avro.NewSpecificDatumWriter()
	writer.SetSchema(schema)

	f, err := os.Create(*outputFile)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	w := bufio.NewWriter(f)
	dfw, err := avro.NewDataFileWriter(w, schema, writer)
	if err != nil {
		panic(err)
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := range objectCh {
			avroObject, err := objectToAvro(i.ProjectId, i.Object)
			if err == nil {
				err = dfw.Write(avroObject)
				if err != nil {
					panic(err)
				}
				dfw.Flush()
			}
		}
	}()
	wg.Wait()

	w.Flush()
	dfw.Close()
	f.Sync()
	glog.Warning("Processing complete.")
}
