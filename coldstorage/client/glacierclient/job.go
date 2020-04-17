package glacierclient

import (
	"io"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/glacier"
	//. "github.com/journeymidnight/yig/coldstorage/client"
	. "github.com/journeymidnight/yig/coldstorage/types/glaciertype"
	. "github.com/journeymidnight/yig/error"
)

//To initiate a job of the specified type, which can be a select, an archival retrieval, or a vault retrieval.
func (c GlacierClient) PostJob(accountid, vaultname, archiveid, snstopic, tier, outputbucket string) (string, error) {
	input := &glacier.InitiateJobInput{
		AccountId:     aws.String(accountid),
		JobParameters: &glacier.JobParameters{
			ArchiveId:	 aws.String(archiveid),
			Description: aws.String(""),
			SNSTopic:    aws.String(snstopic),
			Tier:		 aws.String(tier),
			Type:        aws.String("archive-retrieval"),
			OutputLocation:	&glacier.OutputLocation{
				S3:		&glacier.S3Location{
					BucketName:	aws.String(outputbucket),
				},
			},
		},
		VaultName:     aws.String(vaultname),
	}
	result, err := c.Client.InitiateJob(input)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case glacier.ErrCodeResourceNotFoundException:
				err = ErrResourceNotFound
			case glacier.ErrCodePolicyEnforcedException:
				err = ErrLimitExceeded
			case glacier.ErrCodeInvalidParameterValueException:
				err = ErrInvalidParameterValue
			case glacier.ErrCodeMissingParameterValueException:
				err = ErrMissingParameterValue
			case glacier.ErrCodeInsufficientCapacityException:
				err = ErrInternalError
			case glacier.ErrCodeServiceUnavailableException:
				err = ErrServiceUnavailable
			default:
				Logger.Println(5, "With error: ", aerr.Error())
			}
		} else {
			Logger.Println(5, "With error: ", aerr.Error())
		}
	}
	jobid := aws.StringValue(result.JobId)
	return jobid, err
}

//To check the status of your job.
func (c GlacierClient) GetJobStatus(accountid string, jobid string, vaultname string) (*JobStatus, error) {
	input := &glacier.DescribeJobInput{
		AccountId: aws.String(accountid),
		JobId:     aws.String(jobid),
		VaultName: aws.String(vaultname),
	}
	result, err := c.Client.DescribeJob(input)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case glacier.ErrCodeResourceNotFoundException:
				err = ErrResourceNotFound
			case glacier.ErrCodeInvalidParameterValueException:
				err = ErrInvalidParameterValue
			case glacier.ErrCodeMissingParameterValueException:
				err = ErrMissingParameterValue
			case glacier.ErrCodeServiceUnavailableException:
				err = ErrServiceUnavailable
			default:
				Logger.Println(5, "With error: ", aerr.Error())
			}
		} else {
			Logger.Println(5, "With error: ", aerr.Error())
		}
	}
	jobstatus := &JobStatus{
		Completed:  aws.BoolValue(result.Completed),
		StatusCode: aws.StringValue(result.StatusCode),
	}
	return jobstatus, err
}

//To download the output of the job you initiated using InitiateJob.
func (c GlacierClient) GetJobOutput(accountid string, jobid string, vaultname string) (io.ReadCloser, error) {
	input := &glacier.GetJobOutputInput{
		AccountId: aws.String(accountid),
		JobId:     aws.String(jobid),
		VaultName: aws.String(vaultname),
	}
	result, err := c.Client.GetJobOutput(input)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case glacier.ErrCodeResourceNotFoundException:
				err = ErrResourceNotFound
			case glacier.ErrCodeInvalidParameterValueException:
				err = ErrInvalidParameterValue
			case glacier.ErrCodeMissingParameterValueException:
				err = ErrMissingParameterValue
			case glacier.ErrCodeServiceUnavailableException:
				err = ErrServiceUnavailable
			default:
				Logger.Println(5, "With error: ", aerr.Error())
			}
		} else {
			Logger.Println(5, "With error: ", aerr.Error())
		}
	}
	body := result.Body
	return body, err
}
