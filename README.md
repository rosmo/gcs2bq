# Google Cloud Storage to BigQuery

## Ever wanted to know what's your organization's average file creation time or size?

This small applications discovers all buckets from a Google Cloud Platform organization, 
then fetches all the objects in those and creates an Avro file containing all the objects 
and their attributes. This can be then imported into BigQuery.

### Building

You can build it either manually, or using the supplied `Dockerfile`:

```bash
export GOOGLE_PROJECT=your-project
docker build -t eu.gcr.io/$GOOGLE_PROJECT/gcs2bq:latest .
docker push eu.gcr.io/$GOOGLE_PROJECT/gcs2bq:latest
```

### Usage

```bash
$ ./gcs2bq -help
Google Cloud Storage object metadata to BigQuery, version 0.1
Usage of ./gcs2bq:
  -alsologtostderr
    	log to standard error as well as files
  -file string
    	output file name (default gcs.avro) (default "gcs.avro")
  -log_backtrace_at value
    	when logging hits line file:N, emit a stack trace
  -log_dir string
    	If non-empty, write log files in this directory
  -logtostderr
    	log to standard error instead of files
  -stderrthreshold value
    	logs at or above this threshold go to stderr
  -v value
    	log level for V logs
  -versions
    	include GCS object versions
  -vmodule value
    	comma-separated list of pattern=N settings for file-filtered logging
```

### BigQuery schema

See file [bigquery.schema](bigquery.schema) for the BigQuery table schema. AVRO
schema is in [gcs2bq.avsc](gcs2bq.avsc).

### Running in GKE as a CronJob

You can deploy the container as a `CronJob` in Google Kubernetes Engine. See the file
[gcs2bq.yaml](gcs2bq.yaml). Replace the environment parameters with values appropriate
for your environment.





